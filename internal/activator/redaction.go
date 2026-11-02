package activator

import (
	"context"
	"errors"
	"strings"

	"quota-activation/internal/host"
	"quota-activation/internal/state"
)

// ErrNetworkFailure 表示宿主模型调用在网络层失败，调用方应按可重试上游故障处理。
var ErrNetworkFailure = errors.New("activator: network failure")

type redactedError struct {
	message string
	cause   error
}

func (e redactedError) Error() string { return e.message }
func (e redactedError) Unwrap() error { return e.cause }

func safeError(err error) error {
	if err == nil {
		return nil
	}
	var statusErr *host.HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr
	}
	message := state.Redact(err.Error())
	if message == "" {
		message = "activation error"
	}
	return redactedError{message: message, cause: err}
}

func safeHostError(err error) error {
	if err == nil {
		return nil
	}
	var statusErr *host.HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return safeError(err)
	}
	if isNetworkFailure(err) {
		return redactedError{message: ErrNetworkFailure.Error(), cause: errors.Join(ErrNetworkFailure, err)}
	}
	message := state.Redact(err.Error())
	if message != "" && strings.Contains(message, "host callback host.model.execute:") {
		return redactedError{message: message, cause: err}
	}
	return redactedError{message: "host model execute failed", cause: err}
}

// IsNetworkFailure 判断错误链是否表示宿主模型调用发生网络层失败。
func IsNetworkFailure(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "connection reset by peer")
}

func isNetworkFailure(err error) bool { return IsNetworkFailure(err) }
