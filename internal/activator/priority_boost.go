package activator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"quota-activation/internal/host"
)

const activationPriorityBoostFloor = 1000

// boostPriorityForSelection 临时抬高目标凭证 priority，确保进入 CPA scheduler 最高候选层。
// 同 provider：current < max 或并列最高(peerAtMax>=2) 时 boost 到 max(max+1, floor1000)；
// 唯一最高 noop；需 boost 但物理 JSON 不完整则 error（纯中文，供界面/执行记录）。
//
// 锁范围（T5 并发互顶修复）：per-provider 锁从 boost 开始持有至 restore 调用结束，
// 覆盖 list→save→confirm→（调用方 execute）→restore，避免同 provider 长期双顶。
// 调用方必须 defer restore；restore 幂等且用 Background 亦可。
func (a *Activator) boostPriorityForSelection(ctx context.Context, authID string) (restore func(context.Context), err error) {
	noop := func(context.Context) {}
	if a == nil || a.host == nil || strings.TrimSpace(authID) == "" {
		return noop, nil
	}

	// 先 list 一次确定 provider，再按 provider 加锁（锁内重新 list，避免 TOCTOU）。
	files, err := a.host.ListAuthFiles(ctx)
	if err != nil {
		return noop, fmt.Errorf("列举凭证文件失败：%w", safeHostError(err))
	}
	target, ok := findAuthFileByID(files, authID)
	if !ok {
		return noop, nil
	}
	provider := strings.ToLower(strings.TrimSpace(firstNonBlank(target.Provider, target.Type)))
	mu := a.providerBoostLock(provider)
	mu.Lock()
	// 持有至 restore：禁止 defer Unlock，否则 execute 期间 peer 可再 boost 双顶。
	held := true
	release := func() {
		if held {
			held = false
			mu.Unlock()
		}
	}

	files, err = a.host.ListAuthFiles(ctx)
	if err != nil {
		release()
		return noop, fmt.Errorf("列举凭证文件失败：%w", safeHostError(err))
	}
	target, ok = findAuthFileByID(files, authID)
	if !ok {
		// 无目标：释放锁，noop restore。
		release()
		return noop, nil
	}
	provider = strings.ToLower(strings.TrimSpace(firstNonBlank(target.Provider, target.Type)))
	current := priorityOfAuthFile(target)
	maxPriority, peerAtMax := providerMaxPriorityStats(files, provider)
	if maxPriority < current {
		maxPriority = current
	}
	if !needsPriorityBoost(current, maxPriority, peerAtMax) {
		// 无需写 priority，但仍串行化会话至 restore，避免 peer 交错 boost。
		return func(context.Context) { release() }, nil
	}
	physicalData, physicalName, err := a.loadPhysicalAuthJSON(ctx, target)
	if err != nil {
		release()
		return noop, err
	}
	displayName := strings.TrimSpace(firstNonBlank(physicalName, target.Name, target.ID, authID))
	if !looksLikeFullAuthDocument(physicalData) {
		release()
		return noop, fmt.Errorf("需要提升优先级，但未能读取完整凭证文件：%s", displayName)
	}
	current = priorityFromJSON(physicalData, current)
	if !needsPriorityBoost(current, maxPriority, peerAtMax) {
		return func(context.Context) { release() }, nil
	}
	boosted := maxPriority + 1
	if boosted < activationPriorityBoostFloor {
		boosted = activationPriorityBoostFloor
	}
	originalData := append([]byte(nil), physicalData...)
	originalPriority := current
	patched, err := withPriority(physicalData, boosted)
	if err != nil {
		release()
		return noop, err
	}
	if !looksLikeFullAuthDocument(patched) {
		release()
		return noop, fmt.Errorf("凭证文档不完整，拒绝写入优先级：%s", displayName)
	}
	name := displayName
	if name == "" {
		release()
		return noop, nil
	}
	if err := a.host.SaveAuthFile(ctx, name, patched); err != nil {
		release()
		return noop, fmt.Errorf("提升凭证优先级失败：%w", safeHostError(err))
	}
	if err := a.confirmBoostedUniqueTop(ctx, target, provider, name, boosted); err != nil {
		// confirm 失败：锁内立即回滚 priority，再释放。
		if payload, restoreErr := withPriority(originalData, originalPriority); restoreErr == nil && looksLikeFullAuthDocument(payload) {
			_ = a.host.SaveAuthFile(ctx, name, payload)
		}
		release()
		return noop, err
	}
	restored := false
	return func(restoreCtx context.Context) {
		if restored {
			return
		}
		restored = true
		if restoreCtx == nil {
			restoreCtx = context.Background()
		}
		// 仍持有同一 mu；写回原始 priority 后释放。
		payload, err := withPriority(originalData, originalPriority)
		if err == nil && looksLikeFullAuthDocument(payload) {
			_ = a.host.SaveAuthFile(restoreCtx, name, payload)
		}
		release()
	}, nil
}

func needsPriorityBoost(current, maxPriority, peerAtMax int) bool {
	if current < maxPriority {
		return true
	}
	return current == maxPriority && peerAtMax >= 2
}

func providerMaxPriorityStats(files []host.AuthFile, provider string) (maxPriority int, peerAtMax int) {
	hasPeer := false
	for _, file := range files {
		if strings.ToLower(strings.TrimSpace(firstNonBlank(file.Provider, file.Type))) != provider {
			continue
		}
		p := priorityOfAuthFile(file)
		if !hasPeer || p > maxPriority {
			maxPriority = p
			peerAtMax = 1
			hasPeer = true
			continue
		}
		if p == maxPriority {
			peerAtMax++
		}
	}
	return maxPriority, peerAtMax
}

// loadPhysicalAuthJSON 在 List 返回 stub 时，通过 GetAuthFile 拉取完整物理凭证 JSON。
// AuthIndex 为空时依次用 Name/ID 再试；成功拿到完整文档再交给 boost 写 priority。
func (a *Activator) loadPhysicalAuthJSON(ctx context.Context, file host.AuthFile) ([]byte, string, error) {
	displayName := strings.TrimSpace(firstNonBlank(file.Name, file.ID, file.AuthIndex))
	if looksLikeFullAuthDocument(file.Data) {
		return append([]byte(nil), file.Data...), displayName, nil
	}
	if a == nil || a.host == nil {
		return append([]byte(nil), file.Data...), displayName, nil
	}
	// List.Data 常为 REAUTH stub；必须尝试 GetAuthFile。authIndex 空则回退 Name/ID。
	keys := uniqueNonBlankKeys(file.AuthIndex, file.Name, file.ID)
	bestData := append([]byte(nil), file.Data...)
	bestName := displayName
	var lastErr error
	for _, key := range keys {
		physical, err := a.host.GetAuthFile(ctx, key)
		if err != nil {
			lastErr = err
			continue
		}
		name := strings.TrimSpace(firstNonBlank(physical.Name, file.Name, file.ID, key))
		if looksLikeFullAuthDocument(physical.Data) {
			return append([]byte(nil), physical.Data...), name, nil
		}
		if len(physical.Data) > len(bestData) {
			bestData = append([]byte(nil), physical.Data...)
			bestName = name
		}
	}
	if lastErr != nil && len(bestData) == 0 {
		if displayName == "" {
			return nil, "", fmt.Errorf("读取物理凭证文件失败")
		}
		return nil, "", fmt.Errorf("读取物理凭证文件失败：%s", displayName)
	}
	return bestData, bestName, nil
}

func uniqueNonBlankKeys(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func priorityFromJSON(data []byte, fallback int) int {
	if len(data) == 0 {
		return fallback
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return fallback
	}
	raw, ok := document["priority"]
	if !ok {
		return fallback
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return fallback
	}
	return value
}

func findAuthFileByID(files []host.AuthFile, authID string) (host.AuthFile, bool) {
	want := strings.TrimSpace(authID)
	for _, file := range files {
		if strings.TrimSpace(file.ID) == want || strings.TrimSpace(file.Name) == want || strings.TrimSpace(file.AuthIndex) == want {
			return file, true
		}
		if len(file.Data) == 0 {
			continue
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(file.Data, &document); err != nil {
			continue
		}
		if authFileMatches(document, want) {
			return file, true
		}
	}
	return host.AuthFile{}, false
}

func priorityOfAuthFile(file host.AuthFile) int {
	if file.Priority != 0 {
		return file.Priority
	}
	if len(file.Data) == 0 {
		return 0
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(file.Data, &document); err != nil {
		return 0
	}
	raw, ok := document["priority"]
	if !ok {
		return 0
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0
	}
	return value
}

func withPriority(data []byte, priority int) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("凭证文档为空，拒绝仅写入优先级")
	}
	document := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("解析凭证文件失败：%w", safeError(err))
	}
	if len(document) == 0 {
		return nil, fmt.Errorf("凭证文档无字段，拒绝仅写入优先级")
	}
	value, err := json.Marshal(priority)
	if err != nil {
		return nil, fmt.Errorf("编码优先级失败：%w", safeError(err))
	}
	document["priority"] = value
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("编码凭证文件失败：%w", safeError(err))
	}
	return encoded, nil
}

// looksLikeFullAuthDocument 判断是否为可安全写 priority 的完整凭证文档。
// 认顶层 token、嵌套 tokens.*，以及 type/provider + token/api_key；
// 拒绝仅 email 的 REAUTH stub。
func looksLikeFullAuthDocument(data []byte) bool {
	if len(data) < 64 {
		return false
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return false
	}
	if authDocumentHasTokenField(document) {
		return true
	}
	// 嵌套 tokens.access_token / refresh_token / id_token / api_key
	if tokensRaw, ok := document["tokens"]; ok {
		var tokens map[string]json.RawMessage
		if json.Unmarshal(tokensRaw, &tokens) == nil && authDocumentHasTokenField(tokens) {
			return true
		}
	}
	// 存在非空 type/provider 且任一 token/api_key 字段（顶层已覆盖；此处防空串 type 误判）
	if authDocumentHasIdentity(document) && authDocumentHasTokenField(document) {
		return true
	}
	return false
}

func authDocumentHasTokenField(document map[string]json.RawMessage) bool {
	for _, key := range []string{"access_token", "refresh_token", "id_token", "api_key"} {
		raw, ok := document[key]
		if !ok || isEmptyJSONValue(raw) {
			continue
		}
		return true
	}
	return false
}

func authDocumentHasIdentity(document map[string]json.RawMessage) bool {
	for _, key := range []string{"type", "provider"} {
		raw, ok := document[key]
		if !ok || isEmptyJSONValue(raw) {
			continue
		}
		return true
	}
	return false
}

func isEmptyJSONValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == `""` {
		return true
	}
	return false
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
