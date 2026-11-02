package management

import (
	"context"
	"encoding/json"
	"time"

	"quota-activation/internal/detector"
	"quota-activation/internal/host"
	"quota-activation/internal/planclaim"
	"quota-activation/internal/quotapayload"
)

func (h *Handler) quotaPayload(file host.AuthFile, provider string, availableModels []string, authID string) any {
	now := h.now().UTC()
	if payload, ok := quotapayload.TrueFromFile(file); ok && !codexFiveHourOnly(provider, authID, payload, now) {
		return json.RawMessage(payload)
	}
	if record, ok := h.store.UsableLatestSuccess(authID, provider); ok {
		if name, limitSeconds, resetAt, ok := quotapayload.WindowFromSuccess(record, now); ok {
			if assembled, ok := assembleQuotaPayload(provider, availableModels, name, limitSeconds, resetAt); ok {
				return assembled
			}
		}
	}
	switch detector.Provider(provider) {
	case detector.ProviderCodex:
		name, limitSeconds, resetAt, ok := quotapayload.CodexWindowForPlan(h.codexPlan(file, authID), now)
		if !ok {
			return map[string]any{}
		}
		assembled, ok := assembleQuotaPayload(provider, availableModels, name, limitSeconds, resetAt)
		if !ok {
			return map[string]any{}
		}
		return assembled
	case detector.ProviderAntigravity:
		name, limitSeconds, resetAt, ok := quotapayload.SyntheticWindow(detector.ProviderAntigravity, now)
		if !ok {
			return map[string]any{}
		}
		assembled, ok := quotapayload.MarshalAntigravity(availableModels, name, limitSeconds, resetAt)
		if !ok {
			return map[string]any{"models": map[string]any{}}
		}
		return assembled
	default:
		return map[string]any{}
	}
}

// codexFiveHourOnly 将 Codex 仅 5h 的真源视为缺失（与 parseCodex 忽略 5h 一致）。
func codexFiveHourOnly(provider string, authID string, payload json.RawMessage, now time.Time) bool {
	if detector.Provider(provider) != detector.ProviderCodex {
		return false
	}
	_, err := detector.Evaluate(detector.ProbeInput{
		AuthID:     authID,
		Provider:   detector.ProviderCodex,
		ObservedAt: now,
		Payload:    payload,
	}, "")
	return err != nil
}

// codexPlan 从物理 JSON 解析套餐；List stub 无 JWT 时补拉 host.GetAuthFile。
func (h *Handler) codexPlan(file host.AuthFile, authID string) planclaim.Type {
	plan := planclaim.FromAuthJSON(file.Data)
	if plan != planclaim.TypeUnknown || h.host == nil {
		return plan
	}
	for _, key := range []string{file.AuthIndex, file.Name, file.ID, authID} {
		if key == "" {
			continue
		}
		physical, err := h.host.GetAuthFile(context.Background(), key)
		if err != nil {
			continue
		}
		if got := planclaim.FromAuthJSON(physical.Data); got != planclaim.TypeUnknown {
			return got
		}
	}
	return planclaim.TypeUnknown
}

func assembleQuotaPayload(provider string, availableModels []string, windowName string, limitSeconds int, resetAt time.Time) (any, bool) {
	raw, ok := quotapayload.Marshal(detector.Provider(provider), availableModels, windowName, limitSeconds, resetAt)
	if !ok {
		return nil, false
	}
	return json.RawMessage(raw), true
}
