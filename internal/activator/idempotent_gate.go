package activator

import (
	"strings"
	"time"

	"quota-activation/internal/detector"
	"quota-activation/internal/state"
)

// cycleAlreadyWokenMessage 硬闸跳过时的用户可见文案（manual/auto 共用）。
const cycleAlreadyWokenMessage = "本周期已唤醒，等待重置"

// PreviousStateFromStore 从 success 记录构造 detector.PreviousState，供 Evaluate 与硬闸共用。
// 无 success 时返回空 PreviousState。HasRemaining 兼容 omitempty（Remaining!=0 视为有快照）。
func PreviousStateFromStore(store *state.Store, authID string, provider detector.Provider) detector.PreviousState {
	if store == nil {
		return detector.PreviousState{}
	}
	cycleKey, remaining, hasRemaining, ok := store.LatestSuccessCycle(authID, string(provider))
	if !ok {
		return detector.PreviousState{}
	}
	if !hasRemaining && remaining != 0 {
		hasRemaining = true
	}
	return detector.PreviousState{
		CycleKey:     cycleKey,
		Remaining:    remaining,
		HasRemaining: hasRemaining,
	}
}

// PreviousStateFromStoreForModelGroup 按 auth+provider+modelGroup 读取 previous；空组只匹配无组记录。
func PreviousStateFromStoreForModelGroup(store *state.Store, authID string, provider detector.Provider, modelGroup detector.ModelGroup) detector.PreviousState {
	if store == nil {
		return detector.PreviousState{}
	}
	cycleKey, remaining, hasRemaining, ok := store.LatestSuccessCycleForModelGroup(authID, string(provider), string(modelGroup))
	if !ok {
		return detector.PreviousState{}
	}
	if !hasRemaining && remaining != 0 {
		hasRemaining = true
	}
	return detector.PreviousState{
		CycleKey:     cycleKey,
		Remaining:    remaining,
		HasRemaining: hasRemaining,
	}
}

// shouldHardSkipActiveCycle 在 normalize 之后、真正 wake 之前拦截重复激活。
// 规则：store 有同 auth+provider UsableLatestSuccess 且 ResetAt.After(now) 时，
// 仅当请求带 HasRemaining&&Remaining>0 且 previous 能证明恢复（previous.Remaining<=0 或 current>previous）才放行；
// 否则 StatusSkipped，不 HTTPDo。Codex 5h success 不参与硬闸。
func shouldHardSkipActiveCycle(store *state.Store, request Request, now time.Time) (bool, string) {
	if store == nil {
		return false, ""
	}
	record, ok := store.UsableLatestSuccessForModelGroup(request.AuthID, string(request.Provider), string(requestModelGroup(request)))
	if !ok || record.ResetAt.IsZero() {
		return false, ""
	}
	if !record.ResetAt.UTC().After(now.UTC()) {
		return false, ""
	}
	// remaining 恢复：current 有正 remaining + previous 有快照且从耗尽/回升。
	if request.HasRemaining && request.Remaining > 0 && recordHasRemainingSnapshot(record) {
		if record.Remaining <= 0 || request.Remaining > record.Remaining {
			return false, ""
		}
	}
	return true, cycleAlreadyWokenMessage
}

func requestModelGroup(request Request) detector.ModelGroup {
	if request.ModelGroup != detector.ModelGroupNone {
		return request.ModelGroup
	}
	if request.Provider != detector.ProviderAntigravity {
		return detector.ModelGroupNone
	}
	inferred, ok := detector.InferModelGroup(request.Model)
	if !ok {
		return detector.ModelGroupNone
	}
	return inferred
}

func recordHasRemainingSnapshot(record state.Record) bool {
	if record.HasRemaining {
		return true
	}
	return record.Remaining != 0
}

// applyHardSkip 填充 skipped 结果字段。
func applyHardSkip(result Result, reason string) Result {
	result.Status = StatusSkipped
	result.Success = false
	result.LastError = strings.TrimSpace(reason)
	if result.LastError == "" {
		result.LastError = cycleAlreadyWokenMessage
	}
	result.Nonce = ""
	return result
}
