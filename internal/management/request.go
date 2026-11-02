package management

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"quota-activation/internal/activator"
	"quota-activation/internal/detector"
	"quota-activation/internal/host"
	"quota-activation/internal/planclaim"
	"quota-activation/internal/quotapayload"
)

type activationRequest struct {
	AuthID           string          `json:"auth_id"`
	Provider         string          `json:"provider"`
	ModelGroup       string          `json:"model_group"`
	Model            string          `json:"model"`
	Prompt           string          `json:"prompt"`
	PreviousCycleKey string          `json:"previous_cycle_key"`
	Disabled         bool            `json:"disabled"`
	QuotaPayload     json.RawMessage `json:"quota_payload"`
}

func decodeActivationRequest(request *http.Request) (activationRequest, error) {
	defer request.Body.Close()
	limited := io.LimitReader(request.Body, 1<<20)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var decoded activationRequest
	if err := decoder.Decode(&decoded); err != nil {
		return activationRequest{}, fmt.Errorf("decode activation request: %w", err)
	}
	decoded.AuthID = strings.TrimSpace(decoded.AuthID)
	decoded.Provider = strings.TrimSpace(decoded.Provider)
	decoded.ModelGroup = strings.TrimSpace(decoded.ModelGroup)
	decoded.Model = strings.TrimSpace(decoded.Model)
	decoded.Prompt = strings.TrimSpace(decoded.Prompt)
	decoded.PreviousCycleKey = strings.TrimSpace(decoded.PreviousCycleKey)
	if decoded.AuthID == "" {
		return activationRequest{}, fmt.Errorf("auth_id is required")
	}
	if len(decoded.QuotaPayload) == 0 || strings.TrimSpace(string(decoded.QuotaPayload)) == "" {
		return activationRequest{}, fmt.Errorf("quota_payload is required")
	}
	return decoded, nil
}

// toActivatorRequest 解析手动激活请求。store 非空时注入 previous 做周期去重；
// !Activate 时返回 ok=false（handler 写 skipped，禁止 400 invalid_request）。
// Codex 因 5h/unknown 无法 Evaluate 时，按套餐合成 monthly/weekly 再评一次。
func (h *Handler) toActivatorRequest(r activationRequest) (activator.Request, bool, error) {
	provider, err := parseProvider(r.Provider)
	if err != nil {
		return activator.Request{}, false, err
	}
	modelGroup, err := parseModelGroup(provider, r.ModelGroup, r.Model)
	if err != nil {
		return activator.Request{}, false, err
	}
	if err := validateManualModelGroup(modelGroup); err != nil {
		return activator.Request{}, false, err
	}
	model := strings.TrimSpace(r.Model)
	observedAt := h.now().UTC()
	previous := activator.PreviousStateFromStoreForModelGroup(h.store, r.AuthID, provider, modelGroup)
	if prevKey := strings.TrimSpace(r.PreviousCycleKey); prevKey != "" && previous.CycleKey == "" {
		previous.CycleKey = prevKey
	}
	input := detector.ProbeInput{
		AuthID:     r.AuthID,
		Provider:   provider,
		Model:      model,
		ObservedAt: observedAt,
		Payload:    r.QuotaPayload,
	}
	decision, err := detector.EvaluateWithPrevious(input, previous)
	if err != nil && provider == detector.ProviderCodex && errors.Is(err, detector.ErrUnknownQuota) {
		if synthesized, ok := synthesizeCodexLongWindow(h.host, r.AuthID, observedAt); ok {
			input.Payload = synthesized
			decision, err = detector.EvaluateWithPrevious(input, previous)
		}
	}
	if err != nil {
		return activator.Request{}, false, fmt.Errorf("evaluate quota: %w", err)
	}
	req := activator.Request{
		AuthID:       r.AuthID,
		Provider:     provider,
		ModelGroup:   modelGroup,
		Window:       decision.Observation.Window,
		CycleKey:     decision.CycleKey,
		Model:        model,
		Prompt:       r.Prompt,
		Disabled:     r.Disabled,
		ObservedAt:   observedAt,
		ResetAt:      decision.Observation.ResetAt,
		Remaining:    decision.Observation.Remaining,
		HasRemaining: decision.Observation.HasRemaining,
	}
	if !decision.Activate {
		return req, false, nil
	}
	return req, true, nil
}

func synthesizeCodexLongWindow(hostClient host.Client, authID string, observedAt time.Time) (json.RawMessage, bool) {
	plan := planclaim.TypeUnknown
	if hostClient != nil {
		if file, getErr := hostClient.GetAuthFile(context.Background(), strings.TrimSpace(authID)); getErr == nil {
			plan = planclaim.FromAuthJSON(file.Data)
		}
	}
	name, limitSeconds, resetAt, ok := quotapayload.CodexWindowForPlan(plan, observedAt)
	if !ok {
		return nil, false
	}
	return quotapayload.Marshal(detector.ProviderCodex, nil, name, limitSeconds, resetAt)
}

func validateManualModelGroup(modelGroup detector.ModelGroup) error {
	switch modelGroup {
	case detector.ModelGroupGemini, detector.ModelGroupClaudeGPT, detector.ModelGroupGemini4Argon, detector.ModelGroupNone:
		return nil
	default:
		return fmt.Errorf("model_group is unsupported")
	}
}

func parseProvider(raw string) (detector.Provider, error) {
	switch detector.Provider(strings.TrimSpace(raw)) {
	case detector.ProviderCodex:
		return detector.ProviderCodex, nil
	case detector.ProviderAntigravity:
		return detector.ProviderAntigravity, nil
	case detector.ProviderUnknown:
		return detector.ProviderUnknown, fmt.Errorf("provider is required")
	default:
		return detector.ProviderUnknown, fmt.Errorf("provider is unsupported")
	}
}

func parseModelGroup(provider detector.Provider, raw string, requestedModel string) (detector.ModelGroup, error) {
	trimmed := strings.TrimSpace(raw)
	if provider != detector.ProviderAntigravity {
		return detector.ModelGroupNone, nil
	}
	if trimmed == "" {
		if strings.TrimSpace(requestedModel) != "" {
			return detector.ModelGroupNone, nil
		}
		return detector.ModelGroupNone, fmt.Errorf("model_group is required for antigravity when model is empty")
	}
	switch detector.ModelGroup(trimmed) {
	case detector.ModelGroupGemini:
		return detector.ModelGroupGemini, nil
	case detector.ModelGroupClaudeGPT:
		return detector.ModelGroupClaudeGPT, nil
	case detector.ModelGroupGemini4Argon:
		return detector.ModelGroupGemini4Argon, nil
	case detector.ModelGroupNone:
		return detector.ModelGroupNone, fmt.Errorf("model_group is required for antigravity when model is empty")
	default:
		return detector.ModelGroupNone, fmt.Errorf("model_group is unsupported")
	}
}
