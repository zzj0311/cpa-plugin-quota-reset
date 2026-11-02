package activator

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"quota-activation/internal/detector"
	"quota-activation/internal/host"
)

// authUnauthorizedMaxAttempts：Codex/Antigravity 直连遇 401 时最多请求次数（含首次）。
const authUnauthorizedMaxAttempts = 3

// authUnauthorizedRetryDelay：连续 401 之间的短间隔（0 亦可；此处 50ms 降低瞬时打爆上游）。
const authUnauthorizedRetryDelay = 50 * time.Millisecond

// authMaterialRetryExtraAttempts：resolveDirectAuthMaterial 拿不到完整材料时的额外尝试次数。
const authMaterialRetryExtraAttempts = 2

// authMaterialRetryDelay：材料重试间隔。
const authMaterialRetryDelay = 200 * time.Millisecond

// wakeDirect 是指定 AuthID 的直连唤醒内核（P1）。
// 流程：GetAuthFile 读物理 JSON → ParseAuthMaterial → 按 provider 构建协议 →
// HTTPDo（超时使用 ActivationRequestTimeout）→ 严格成功判定 → 填充 Result。
//
// 本路径禁止：SaveAuthFile、ModelExecute、scheduler session/nonce、priority boost。
// 供 T4 将 Activate 默认切换到 direct_http 时调用；本波次不改 Activate 默认路径。
func (a *Activator) wakeDirect(ctx context.Context, request Request, result Result) (Result, error) {
	if a == nil || a.host == nil {
		return a.failDirect(result, "直连唤醒失败：缺少宿主依赖"), nil
	}

	authID := strings.TrimSpace(request.AuthID)
	if authID == "" {
		return a.failDirect(result, "直连唤醒失败：缺少凭证标识"), nil
	}

	callCtx := ctx
	cancel := func() {}
	if a.config.ActivationRequestTimeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, a.config.ActivationRequestTimeout)
	}
	defer cancel()

	// 管理面 AuthID 多为凭证文件名；直连只需可解析 token（物理 JSON 或 runtime）。
	materialData, err := a.resolveDirectAuthMaterial(callCtx, authID)
	if err != nil {
		return a.failDirect(result, directMaterialError(request.Provider, err)), nil
	}
	if len(materialData) == 0 {
		return a.failDirect(result, directMaterialError(request.Provider, fmt.Errorf("直连唤醒失败：凭证内容为空"))), nil
	}

	material, err := ParseAuthMaterial(materialData)
	if err != nil {
		return a.failDirect(result, chineseDirectError(err, "直连唤醒失败：凭证材料无效")), nil
	}

	model := strings.TrimSpace(request.Model)
	if model == "" {
		return a.failDirect(result, "直连唤醒失败：缺少模型名称"), nil
	}
	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(a.config.ActivationPrompt)
	}

	protocol, err := buildDirectProtocol(request.Provider, material, model, prompt)
	if err != nil {
		return a.failDirect(result, chineseDirectError(err, "直连唤醒失败：协议构建失败")), nil
	}

	httpReq := protocol.ToHTTPRequest()
	var response host.HTTPResponse
	unauthorizedAttempts := 0
	for attempt := 1; attempt <= authUnauthorizedMaxAttempts; attempt++ {
		var httpErr error
		response, httpErr = a.host.HTTPDo(callCtx, httpReq)
		if httpErr != nil {
			result.HTTPStatus = statusCodeFromError(httpErr)
			return a.failDirect(result, "直连唤醒失败：上游请求失败"), nil
		}
		result.HTTPStatus = response.StatusCode
		if response.StatusCode != http.StatusUnauthorized {
			break
		}
		unauthorizedAttempts++
		if unauthorizedAttempts >= authUnauthorizedMaxAttempts {
			return a.skipDirect(result, authUnauthorizedSkipMessage(request.Provider)), nil
		}
		// 短间隔后重试；ctx 取消则立即失败。
		if err := sleepWithContext(callCtx, authUnauthorizedRetryDelay); err != nil {
			result.HTTPStatus = http.StatusUnauthorized
			return a.failDirect(result, "直连唤醒失败：上游请求失败"), nil
		}
	}

	ok, message := evaluateDirectSuccess(request.Provider, response.StatusCode, response.Body)
	if !ok {
		if message == "" {
			message = "直连唤醒失败：上游判定未通过"
		}
		return a.failDirect(result, message), nil
	}

	result.Status = StatusSuccess
	result.Success = true
	result.LastError = ""
	// 直连路径不产生 scheduler nonce。
	result.Nonce = ""
	return result, nil
}

func (a *Activator) failDirect(result Result, message string) Result {
	result.Status = StatusFailed
	result.Success = false
	result.LastError = strings.TrimSpace(message)
	result.Nonce = ""
	return result
}

// skipDirect：鉴权失效等不可恢复场景，记为 skipped（不计入失败统计、不触发 boost fallback）。
func (a *Activator) skipDirect(result Result, message string) Result {
	result.Status = StatusSkipped
	result.Success = false
	result.LastError = strings.TrimSpace(message)
	result.Nonce = ""
	return result
}

// authUnauthorizedSkipMessage 三次 401 后的用户可见文案（品牌英文 + 中文说明）。
func authUnauthorizedSkipMessage(provider detector.Provider) string {
	switch provider {
	case detector.ProviderAntigravity:
		return "Antigravity唤醒失败：凭证已失效或鉴权失败（已重试3次）"
	default:
		return "Codex唤醒失败：凭证已失效或鉴权失败（已重试3次）"
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *Activator) sleepCtx(ctx context.Context, d time.Duration) bool {
	if a != nil && a.sleep != nil {
		return a.sleep(ctx, d)
	}
	return sleepWithContext(ctx, d) == nil
}

// resolveDirectAuthMaterial 解析直连唤醒所需凭证材料（只需 token，不必完整物理文档）。
// 顺序：list 匹配 → 多 key GetAuthFile → GetRuntimeAuthFile → 直接 GetAuthFile(authID)。
// 宿主索引未就绪时短重试最多 extra 次。失败返回品牌无关纯中文。
func (a *Activator) resolveDirectAuthMaterial(ctx context.Context, authID string) ([]byte, error) {
	authID = strings.TrimSpace(authID)
	if authID == "" {
		return nil, fmt.Errorf("直连唤醒失败：缺少凭证标识")
	}
	if a == nil || a.host == nil {
		return nil, fmt.Errorf("直连唤醒失败：缺少宿主依赖")
	}

	data, err := a.loadDirectAuthMaterial(ctx, authID)
	if err == nil {
		return data, nil
	}
	lastErr := err
	for attempt := 0; attempt < authMaterialRetryExtraAttempts; attempt++ {
		if !a.sleepCtx(ctx, authMaterialRetryDelay) {
			return nil, lastErr
		}
		data, err = a.loadDirectAuthMaterial(ctx, authID)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (a *Activator) loadDirectAuthMaterial(ctx context.Context, authID string) ([]byte, error) {
	tryData := func(data []byte) ([]byte, bool) {
		if len(data) == 0 {
			return nil, false
		}
		if looksLikeFullAuthDocument(data) {
			return append([]byte(nil), data...), true
		}
		if _, err := ParseAuthMaterial(data); err == nil {
			return append([]byte(nil), data...), true
		}
		return nil, false
	}

	var listTarget host.AuthFile
	var listHit bool
	if files, listErr := a.host.ListAuthFiles(ctx); listErr == nil {
		if target, ok := findAuthFileByID(files, authID); ok {
			listTarget, listHit = target, true
			// loadPhysical 错误仅供 boost；直连忽略「读取物理…」文案，继续多 key 回退。
			data, _, err := a.loadPhysicalAuthJSON(ctx, target)
			if err == nil {
				if out, ok := tryData(data); ok {
					return out, nil
				}
			}
			if out, ok := tryData(target.Data); ok {
				return out, nil
			}
		}
	}

	// list 命中后对所有 unique keys 调 GetAuthFile + GetRuntimeAuthFile（不仅 AuthIndex）。
	keys := uniqueNonBlankKeys(authID)
	if listHit {
		keys = uniqueNonBlankKeys(listTarget.AuthIndex, listTarget.Name, listTarget.ID, authID)
	}
	for _, key := range keys {
		if file, err := a.host.GetAuthFile(ctx, key); err == nil {
			if out, ok := tryData(file.Data); ok {
				return out, nil
			}
		}
		if runtimeFile, err := a.host.GetRuntimeAuthFile(ctx, key); err == nil {
			if out, ok := tryData(runtimeFile.Data); ok {
				return out, nil
			}
		}
	}

	return nil, fmt.Errorf("无法读取凭证材料")
}

// resolveDirectAuthJSON 兼容旧调用：等价于 resolveDirectAuthMaterial。
func (a *Activator) resolveDirectAuthJSON(ctx context.Context, authID string) ([]byte, error) {
	return a.resolveDirectAuthMaterial(ctx, authID)
}

// directMaterialError 将取证失败映射为「Codex/Antigravity直连唤醒失败：无法读取凭证材料」。
func directMaterialError(provider detector.Provider, err error) string {
	brand := "Codex"
	if provider == detector.ProviderAntigravity {
		brand = "Antigravity"
	}
	base := brand + "直连唤醒失败：无法读取凭证材料"
	if err == nil {
		return base
	}
	msg := strings.TrimSpace(err.Error())
	// 禁止把物理凭证 / 英文宿主错误冒成用户主文案。
	if msg == "" || strings.Contains(msg, "读取物理凭证文件失败") || containsLatinLetter(msg) {
		return base
	}
	if strings.Contains(msg, "无法读取凭证材料") || strings.Contains(msg, "凭证内容为空") {
		return base
	}
	if strings.Contains(msg, "缺少凭证标识") || strings.Contains(msg, "缺少宿主依赖") {
		return chineseDirectError(err, base)
	}
	return base
}

func buildDirectProtocol(provider detector.Provider, material AuthMaterial, model, prompt string) (ProtocolRequest, error) {
	switch provider {
	case detector.ProviderCodex:
		return BuildCodexProtocol(material, model, prompt)
	case detector.ProviderAntigravity:
		return BuildAntigravityProtocol(material, model, prompt)
	default:
		return ProtocolRequest{}, fmt.Errorf("直连唤醒失败：不支持的提供方")
	}
}

func evaluateDirectSuccess(provider detector.Provider, statusCode int, body []byte) (bool, string) {
	switch provider {
	case detector.ProviderCodex:
		return EvaluateCodexActivationSuccess(statusCode, body)
	case detector.ProviderAntigravity:
		return EvaluateAntigravityActivationSuccess(statusCode, body)
	default:
		if !host.IsHTTPSuccess(statusCode) {
			return false, "直连唤醒失败：上游返回非成功状态"
		}
		return false, "直连唤醒失败：不支持的提供方"
	}
}

// chineseDirectError 提取纯中文用户可见错误；若原错误已含中文则直接使用。
func chineseDirectError(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return fallback
	}
	// 协议/凭证错误已是中文包装；禁止把英文技术串写入 LastError。
	if containsLatinLetter(msg) {
		// 尝试剥掉英文 error 前缀后的中文后缀（如 "凭证材料无效：缺少访问令牌"）。
		if idx := strings.Index(msg, "："); idx >= 0 {
			tail := strings.TrimSpace(msg[idx+len("："):])
			if tail != "" && !containsLatinLetter(tail) {
				return fallback + "：" + tail
			}
		}
		if idx := strings.Index(msg, ":"); idx >= 0 {
			// 兼容 errors.New 英文前缀 + 中文包装（少见）。
			tail := strings.TrimSpace(msg[idx+1:])
			if tail != "" && !containsLatinLetter(tail) {
				return tail
			}
		}
		return fallback
	}
	return msg
}

func containsLatinLetter(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}
