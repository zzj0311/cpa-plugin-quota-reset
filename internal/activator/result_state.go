package activator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"quota-activation/internal/state"
)

// saveStateTimeout 状态落盘独立超时，避免父请求 cancel 后永久挂起。
const saveStateTimeout = 30 * time.Second

// saveStateWarningCanceled 激活已成功但状态保存被中断时的中文提示（非凭证失效）。
const saveStateWarningCanceled = "唤醒已成功，但状态保存被中断（内部超时/取消），非凭证失效"

// saveStateWarningGeneric 激活已成功但状态保存因其它原因失败时的中文提示前缀。
const saveStateWarningGeneric = "唤醒已成功，但状态保存失败（非凭证失效）"

func (a *Activator) failAndStore(ctx context.Context, result Result, err error) (Result, error) {
	result.Status = failedStatus(result.Status)
	result.Success = false
	// LastError 对用户/历史可见：脱敏后映射为纯中文。
	result.LastError = LocalizeUserMessage(state.Redact(err.Error()))
	stored, storeErr := a.storeResult(ctx, result)
	if storeErr != nil {
		return stored, errors.Join(err, storeErr)
	}
	return stored, err
}

func failedStatus(status Status) Status {
	if status == StatusSkipped || status == StatusBusy {
		return status
	}
	return StatusFailed
}

func (a *Activator) storeResult(ctx context.Context, result Result) (Result, error) {
	if a.store == nil {
		return result, ErrMissingDependency
	}
	rec := state.Record{
		AuthID:       result.AuthID,
		Provider:     result.Provider,
		Window:       result.Window,
		CycleKey:     result.CycleKey,
		ObservedAt:   result.ObservedAt,
		ResetAt:      result.ResetAt,
		LastResult:   string(result.Status),
		LastError:    result.LastError,
		Remaining:    result.Remaining,
		HasRemaining: result.HasRemaining,
	}
	a.store.Upsert(rec)
	if result.Status == StatusSuccess && result.Success {
		a.rememberUsableSuccess(rec)
	}
	// 持久化必须不受父 cancel 影响：唤醒网络已成功时，父 scan/HTTP ctx 常已取消。
	saveCtx, cancel := detachSaveContext(ctx)
	defer cancel()
	if err := a.store.SaveAtomic(saveCtx); err != nil {
		// 激活主流程已成功：仅警告，不把整条标失败（避免误导为凭证失效）。
		if result.Status == StatusSuccess && result.Success {
			result.Warning = formatSaveStateWarning(err)
			return result, nil
		}
		return result, fmt.Errorf("save activation state: %w", safeError(err))
	}
	return result, nil
}

// detachSaveContext 返回不受父 cancel 影响、带独立超时的写盘上下文。
func detachSaveContext(parent context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if parent != nil {
		// WithoutCancel 保留 parent 的 values，但不传播 cancel。
		base = context.WithoutCancel(parent)
	}
	return context.WithTimeout(base, saveStateTimeout)
}

// formatSaveStateWarning 生成用户可见的中文状态保存警告（非凭证失效）。
func formatSaveStateWarning(err error) string {
	if err == nil {
		return saveStateWarningGeneric
	}
	if isContextInterrupt(err) {
		return saveStateWarningCanceled
	}
	msg := state.Redact(err.Error())
	if msg == "" {
		return saveStateWarningGeneric
	}
	return saveStateWarningGeneric + "：" + msg
}

func isContextInterrupt(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "context canceled") ||
		strings.Contains(lower, "context cancelled") ||
		strings.Contains(lower, "context deadline exceeded")
}
