package runtime

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"quota-activation/internal/activator"
	"quota-activation/internal/state"
)

const maxRunHistory = 5

const (
	runHistoryKindActivation = "activation"
	runHistoryKindScan       = "scan"
	runHistoryTriggerManual  = "manual"
	runHistoryTriggerAuto    = "auto"
)

type scanSummaryInput struct {
	Attempted   int
	Succeeded   int
	Failed      int
	Skipped     int
	SkipReasons map[string]int
	FailReasons map[string]int
	Providers   []RunHistoryProvider
	Warning     string
}

type scanSummaryAccumulator struct {
	mu          sync.Mutex
	attempted   int
	succeeded   int
	failed      int
	skipped     int
	skipReasons map[string]int
	failReasons map[string]int
	byProvider  map[string]*RunHistoryProvider
	warning     string
}

func newScanSummaryAccumulator() *scanSummaryAccumulator {
	return &scanSummaryAccumulator{
		skipReasons: make(map[string]int),
		failReasons: make(map[string]int),
		byProvider:  make(map[string]*RunHistoryProvider),
	}
}

func (a *scanSummaryAccumulator) add(provider string, detail autoScanDetail) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.attempted++
	name := strings.TrimSpace(provider)
	if name == "" {
		name = "unknown"
	}
	p := a.byProvider[name]
	if p == nil {
		p = &RunHistoryProvider{Name: name}
		a.byProvider[name] = p
	}
	p.Attempted++
	switch detail.outcome {
	case autoScanOutcomeSucceeded:
		a.succeeded++
		p.Succeeded++
	case autoScanOutcomeFailed:
		a.failed++
		p.Failed++
		reason := strings.TrimSpace(detail.errMessage)
		if reason == "" {
			reason = strings.TrimSpace(detail.reason)
		}
		if reason == "" {
			reason = "唤醒失败"
		}
		reason = localizeHistoryMessage(reason)
		a.failReasons[reason]++
		if p.Error == "" {
			p.Error = reason
		}
	case autoScanOutcomeSkipped:
		a.skipped++
		p.Skipped++
		reason := strings.TrimSpace(detail.reason)
		if reason != "" {
			a.skipReasons[reason]++
		}
	}
	if a.warning == "" && isSaveStateWarningText(detail.warning) {
		a.warning = strings.TrimSpace(detail.warning)
	}
}

func (a *scanSummaryAccumulator) toInput() scanSummaryInput {
	a.mu.Lock()
	defer a.mu.Unlock()
	providers := make([]RunHistoryProvider, 0, len(a.byProvider))
	for _, p := range a.byProvider {
		providers = append(providers, *p)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})
	return scanSummaryInput{
		Attempted: a.attempted, Succeeded: a.succeeded, Failed: a.failed, Skipped: a.skipped,
		SkipReasons: a.skipReasons, FailReasons: a.failReasons, Providers: providers,
		Warning: a.warning,
	}
}

// snapshotActivation 仅用于 manual 唤醒记录。
func (r *Runtime) snapshotActivation(trigger string, result activator.Result, err error) {
	at := r.nowUTC()
	entry := buildActivationHistoryEntry(trigger, result, err, at)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prependRunHistoryLocked(entry)
}

// snapshotScanSummary 一轮 auto tick 只写这一条合并摘要。
func (r *Runtime) snapshotScanSummary(in scanSummaryInput) {
	at := r.nowUTC()
	entry := buildScanSummaryHistoryEntry(in, at)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prependRunHistoryLocked(entry)
}

func buildScanSummaryHistoryEntry(in scanSummaryInput, at time.Time) RunHistoryEntry {
	return RunHistoryEntry{
		At: at, Kind: runHistoryKindScan, Trigger: runHistoryTriggerAuto,
		Attempted: in.Attempted, Succeeded: in.Succeeded, Failed: in.Failed, Skipped: in.Skipped,
		Providers: in.Providers, Message: formatScanSummaryMessage(in),
	}
}

func formatScanSummaryMessage(in scanSummaryInput) string {
	warning := strings.TrimSpace(in.Warning)
	if warning == "" || !isSaveStateWarningText(warning) {
		return ""
	}
	return warning
}

func formatTopReasons(label string, reasons map[string]int, maxN int) string {
	if len(reasons) == 0 || maxN <= 0 {
		return ""
	}
	type reasonCount struct {
		reason string
		count  int
	}
	items := make([]reasonCount, 0, len(reasons))
	for reason, count := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" || count <= 0 {
			continue
		}
		items = append(items, reasonCount{reason: localizeSkipReason(reason), count: count})
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].reason < items[j].reason
	})
	if len(items) > maxN {
		items = items[:maxN]
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s ×%d", item.reason, item.count))
	}
	return label + "：" + strings.Join(parts, "，")
}

func localizeSkipReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	switch {
	case trimmed == "":
		return trimmed
	case strings.Contains(trimmed, "额度周期已处理") ||
		strings.Contains(trimmed, "本周期已唤醒") ||
		strings.Contains(trimmed, "quota 周期已处理") ||
		strings.EqualFold(trimmed, "quota cycle already processed"):
		return "额度周期已处理"
	case strings.Contains(trimmed, "额度已恢复") ||
		strings.Contains(trimmed, "quota remaining 恢复"):
		return "额度已恢复"
	case strings.Contains(trimmed, "额度周期可用") ||
		strings.Contains(trimmed, "quota 周期可用"):
		return "额度周期可用"
	case strings.Contains(trimmed, "remaining") && (strings.Contains(trimmed, "可激活") || strings.Contains(strings.ToLower(trimmed), "activat")):
		return "额度可激活"
	case strings.EqualFold(trimmed, "busy") || trimmed == "执行中":
		return "执行中"
	case strings.Contains(trimmed, "调度未选中目标凭证") || strings.Contains(trimmed, "调度器未选中目标凭证"):
		return "调度未选中目标凭证（已冷却）"
	case isHostExecutorUnavailableError(trimmed) || strings.Contains(trimmed, hostExecutorUnavailableCooldownReason):
		return hostExecutorUnavailableCooldownReason
	default:
		return localizeHistoryMessage(trimmed)
	}
}

func (r *Runtime) currentRunHistory() []RunHistoryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.runHistory) == 0 {
		return nil
	}
	out := make([]RunHistoryEntry, len(r.runHistory))
	copy(out, r.runHistory)
	return out
}

func (r *Runtime) prependRunHistoryLocked(entry RunHistoryEntry) {
	history := make([]RunHistoryEntry, 0, maxRunHistory)
	history = append(history, entry)
	for i := 0; i < len(r.runHistory) && len(history) < maxRunHistory; i++ {
		history = append(history, r.runHistory[i])
	}
	r.runHistory = history
}

func (r *Runtime) nowUTC() time.Time {
	if r.now != nil {
		return r.now().UTC()
	}
	return time.Now().UTC()
}

func buildActivationHistoryEntry(trigger string, result activator.Result, err error, at time.Time) RunHistoryEntry {
	if trigger != runHistoryTriggerManual && trigger != runHistoryTriggerAuto {
		trigger = runHistoryTriggerManual
	}
	attempted, succeeded, failed, skipped := 1, 0, 0, 0
	switch result.Status {
	case activator.StatusSuccess:
		if result.Success {
			succeeded = 1
		} else {
			failed = 1
		}
	case activator.StatusSkipped, activator.StatusBusy:
		skipped = 1
	default:
		failed = 1
	}

	providerErr := ""
	if result.LastError != "" {
		providerErr = localizeHistoryMessage(state.Redact(result.LastError))
	} else if err != nil {
		providerErr = localizeHistoryMessage(state.Redact(err.Error()))
	}

	warning := strings.TrimSpace(result.Warning)
	if warning == "" && succeeded == 1 && err != nil {
		warning = localizeHistoryMessage(state.Redact(err.Error()))
		if !isSaveStateWarningText(warning) {
			warning = ""
		}
	}

	providerName := state.Redact(result.Provider)
	message := ""
	if warning != "" && succeeded == 1 {
		message = warning
		if providerErr == "" {
			providerErr = warning
		}
	} else if providerErr != "" {
		message = providerErr
	}

	return RunHistoryEntry{
		At: at, Kind: runHistoryKindActivation, Trigger: trigger,
		Attempted: attempted, Succeeded: succeeded, Failed: failed, Skipped: skipped,
		Providers: []RunHistoryProvider{{
			Name: providerName, Attempted: 1, Succeeded: succeeded, Failed: failed, Skipped: skipped, Error: providerErr,
		}},
		Message:  message,
		WakePath: string(result.WakePath),
	}
}

func localizeHistoryMessage(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return msg
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "save activation state") ||
		strings.Contains(lower, "save state context") ||
		(strings.Contains(lower, "context canceled") && strings.Contains(lower, "save")) ||
		(strings.Contains(lower, "context cancelled") && strings.Contains(lower, "save")):
		return "唤醒已成功，但状态保存被中断（内部超时/取消），非凭证失效"
	case strings.Contains(lower, "context canceled") || strings.Contains(lower, "context cancelled"):
		return "唤醒已成功，但状态保存被中断（内部超时/取消），非凭证失效"
	case strings.Contains(lower, "activation scheduler did not select target credential") ||
		strings.Contains(msg, "调度器未选中目标凭证"):
		return "调度器未选中目标凭证"
	case strings.Contains(lower, "antigravity activation response missing choices"):
		return "Antigravity 唤醒响应缺少选项"
	case isHostExecutorUnavailableError(msg):
		return hostExecutorUnavailableMessage
	case strings.Contains(lower, "host model execute") && strings.Contains(lower, "unavailable"):
		return hostExecutorUnavailableMessage
	// 旧英文 boost/confirm 错误 → 纯中文（生产路径已改中文，此处兜底历史记录）
	case strings.Contains(lower, "priority boost required but auth document incomplete"):
		return "需要提升优先级，但未能读取完整凭证文件"
	case strings.Contains(lower, "priority boost refused incomplete auth document"):
		return "凭证文档不完整，拒绝写入优先级"
	case strings.Contains(lower, "is not unique highest priority") ||
		strings.Contains(lower, "runtime priority") && strings.Contains(lower, "expected") ||
		strings.Contains(lower, "priority boost confirm failed") ||
		strings.Contains(lower, "runtime auth not found"):
		return "优先级提升后，运行时仍未确认目标为最高优先级"
	case strings.Contains(lower, "priority boost confirm canceled"):
		return "优先级确认已取消"
	case strings.Contains(lower, "priority boost confirm: missing host"):
		return "优先级确认失败：宿主不可用"
	case strings.Contains(lower, "get physical auth file") ||
		strings.Contains(lower, "list auth files for priority boost"):
		return "读取凭证文件失败"
	case strings.Contains(lower, "boost auth priority"):
		return "提升凭证优先级失败"
	case strings.Contains(lower, "auth document is empty") ||
		strings.Contains(lower, "auth document has no fields"):
		return "凭证文档不完整，拒绝写入优先级"
	case strings.Contains(lower, "usage_limit") || strings.Contains(lower, "usage limit"):
		return "Codex唤醒失败：用量额度已耗尽"
	case strings.Contains(lower, "create activation session"):
		return "创建唤醒会话失败"
	case strings.Contains(lower, "available model is required"):
		return "缺少可用模型"
	case strings.Contains(lower, "host model execute failed") ||
		strings.Contains(lower, "host model execute") ||
		strings.Contains(lower, "model execute failed"):
		return "宿主模型执行失败"
	case strings.Contains(lower, "activation error") || strings.Contains(lower, "activator: missing dependency"):
		return "唤醒失败"
	case strings.Contains(lower, "activator: busy") || strings.EqualFold(msg, "busy"):
		return "执行中"
	case strings.Contains(lower, "missing auth_id") || strings.Contains(lower, "invalid request") ||
		strings.HasPrefix(lower, "missing "):
		return "唤醒请求无效：缺少必要字段"
	case strings.Contains(lower, "connection reset by peer") || strings.Contains(lower, "network failure"):
		return "网络连接失败"
	default:
		// 兜底：委托 activator 统一映射，避免历史仍残留英文。
		return activator.LocalizeUserMessage(msg)
	}
}

func isSaveStateWarningText(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(msg, "状态保存") ||
		strings.Contains(lower, "save activation state") ||
		strings.Contains(lower, "save state context") ||
		strings.Contains(lower, "context canceled") ||
		strings.Contains(lower, "context cancelled")
}
