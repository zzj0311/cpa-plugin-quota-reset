package quotapayload

import (
	"encoding/json"
	"strings"
	"time"

	"quota-activation/internal/detector"
	"quota-activation/internal/host"
	"quota-activation/internal/planclaim"
	"quota-activation/internal/state"
)

// TrueFromFile 读取 runtime/file 上的真实 quota_payload，不做合成。
func TrueFromFile(file host.AuthFile) (json.RawMessage, bool) {
	var document struct {
		QuotaPayload      json.RawMessage `json:"quota_payload"`
		QuotaPayloadUpper json.RawMessage `json:"quotaPayload"`
	}
	if err := json.Unmarshal(file.Data, &document); err == nil {
		for _, raw := range []json.RawMessage{document.QuotaPayload, document.QuotaPayloadUpper} {
			if len(raw) > 0 {
				return append([]byte(nil), raw...), true
			}
		}
	}
	for _, values := range []map[string]any{file.Metadata, file.Attributes} {
		for _, key := range []string{"quota_payload", "quotaPayload"} {
			encoded, ok := encode(values[key])
			if ok {
				return encoded, true
			}
		}
	}
	return nil, false
}

// WindowFromSuccess 用 LatestSuccess 同窗推断；ResetAt 已过期则拒绝。
func WindowFromSuccess(record state.Record, observedAt time.Time) (name string, limitSeconds int, resetAt time.Time, ok bool) {
	observedAt = observedAt.UTC()
	if !record.ResetAt.IsZero() && !record.ResetAt.UTC().After(observedAt) {
		return "", 0, time.Time{}, false
	}
	name, limitSeconds, ok = normalizeWindow(record.Window)
	if !ok {
		return "", 0, time.Time{}, false
	}
	resetAt = record.ResetAt.UTC()
	if resetAt.IsZero() {
		resetAt = StableResetAt(observedAt, time.Duration(limitSeconds)*time.Second)
	}
	return name, limitSeconds, resetAt, true
}

// CodexWindowForPlan 按套餐返回 Codex 长窗（禁止 5h）。
// TypePaid → weekly 7d；TypeFree / TypeUnknown / 其它 → monthly 30d。
func CodexWindowForPlan(plan planclaim.Type, observedAt time.Time) (name string, limitSeconds int, resetAt time.Time, ok bool) {
	observedAt = observedAt.UTC()
	name, limitSeconds = planclaim.WindowFor(plan)
	resetAt = StableResetAt(observedAt, time.Duration(limitSeconds)*time.Second)
	return name, limitSeconds, resetAt, true
}

// SyntheticWindow 提供商默认窗：Codex 默认 monthly 30d，AG=weekly 7d。
func SyntheticWindow(provider detector.Provider, observedAt time.Time) (name string, limitSeconds int, resetAt time.Time, ok bool) {
	observedAt = observedAt.UTC()
	switch provider {
	case detector.ProviderCodex:
		return CodexWindowForPlan(planclaim.TypeUnknown, observedAt)
	case detector.ProviderAntigravity:
		return "weekly", 7 * 24 * 60 * 60, StableResetAt(observedAt, 7*24*time.Hour), true
	default:
		return "", 0, time.Time{}, false
	}
}

// Marshal 按提供商编码 quota_payload（AG 可多模型同窗）。
func Marshal(provider detector.Provider, models []string, windowName string, limitSeconds int, resetAt time.Time) ([]byte, bool) {
	resetText := resetAt.UTC().Format(time.RFC3339)
	switch provider {
	case detector.ProviderCodex:
		payload, err := json.Marshal(map[string]any{
			"reset_at":             resetText,
			"limit_window_seconds": limitSeconds,
			"name":                 windowName,
		})
		return payload, err == nil
	case detector.ProviderAntigravity:
		encoded, ok := MarshalAntigravity(models, windowName, limitSeconds, resetAt)
		if !ok {
			return nil, false
		}
		payload, err := json.Marshal(encoded)
		return payload, err == nil
	default:
		return nil, false
	}
}

// MarshalAntigravity 为每个可识别模型写入同一计量窗。
func MarshalAntigravity(models []string, windowName string, limitSeconds int, resetAt time.Time) (map[string]any, bool) {
	resetText := resetAt.UTC().Format(time.RFC3339)
	encoded := map[string]any{}
	for _, model := range models {
		group, ok := antigravityGroup(model)
		if !ok {
			continue
		}
		providerName := "anthropic"
		if group == detector.ModelGroupGemini || group == detector.ModelGroupGemini4Argon {
			providerName = "gemini"
		}
		encoded[model] = map[string]any{
			"modelProvider": providerName,
			"quotaInfo": map[string]any{
				"windows": []map[string]any{{
					"resetTime":            resetText,
					"name":                 windowName,
					"limit_window_seconds": limitSeconds,
				}},
			},
		}
	}
	if len(encoded) == 0 {
		return nil, false
	}
	return map[string]any{"models": encoded}, true
}

// StableResetAt 将 observedAt 截断到窗长对齐边界再加一窗。
func StableResetAt(observedAt time.Time, window time.Duration) time.Time {
	if window <= 0 {
		window = 7 * 24 * time.Hour
	}
	now := observedAt.UTC()
	return now.Truncate(window).Add(window)
}

func normalizeWindow(raw string) (name string, limitSeconds int, ok bool) {
	switch detector.Window(strings.ToLower(strings.TrimSpace(raw))) {
	case detector.WindowMonthly:
		return "monthly", 30 * 24 * 60 * 60, true
	case detector.WindowWeekly:
		return "weekly", 7 * 24 * 60 * 60, true
	case detector.WindowFiveHour:
		return "5h", 5 * 60 * 60, true
	default:
		text := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case strings.Contains(text, "month") || strings.Contains(text, "30d"):
			return "monthly", 30 * 24 * 60 * 60, true
		case strings.Contains(text, "week") || strings.Contains(text, "7d"):
			return "weekly", 7 * 24 * 60 * 60, true
		case strings.Contains(text, "5h") || strings.Contains(text, "5 hour"):
			return "5h", 5 * 60 * 60, true
		default:
			return "", 0, false
		}
	}
}

func antigravityGroup(model string) (detector.ModelGroup, bool) {
	return detector.InferModelGroup(model)
}

func encode(raw any) (json.RawMessage, bool) {
	switch value := raw.(type) {
	case nil:
		return nil, false
	case json.RawMessage:
		return append([]byte(nil), value...), len(value) > 0
	case []byte:
		return append([]byte(nil), value...), len(value) > 0
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, false
		}
		return json.RawMessage(trimmed), true
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		return encoded, true
	}
}
