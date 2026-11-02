package activator

import (
	"fmt"
	"strings"

	"quota-activation/internal/detector"
)

func (a *Activator) normalizeRequest(request Request) (Request, error) {
	normalized := request
	normalized.AuthID = strings.TrimSpace(request.AuthID)
	normalized.Model = strings.TrimSpace(request.Model)
	normalized.Prompt = strings.TrimSpace(request.Prompt)
	if normalized.Prompt == "" {
		normalized.Prompt = strings.TrimSpace(a.config.ActivationPrompt)
	}
	missing := missingRequestFields(normalized)
	if len(missing) > 0 {
		return normalized, fmt.Errorf("missing %s: %w", strings.Join(missing, ","), ErrInvalidRequest)
	}
	return normalized, nil
}

func missingRequestFields(request Request) []string {
	var missing []string
	if request.AuthID == "" {
		missing = append(missing, "auth_id")
	}
	if strings.TrimSpace(request.CycleKey.String()) == "" {
		missing = append(missing, "cycle_key")
	}
	if request.Provider == "" || request.Provider == detector.ProviderUnknown {
		missing = append(missing, "provider")
	}
	if request.Window == "" || request.Window == detector.WindowUnknown {
		missing = append(missing, "window")
	}
	return missing
}

func (a *Activator) initialResult(request Request) Result {
	observedAt := request.ObservedAt
	if observedAt.IsZero() {
		observedAt = a.now().UTC()
	}
	return Result{
		AuthID:       request.AuthID,
		Provider:     string(request.Provider),
		Window:       string(request.Window),
		CycleKey:     request.CycleKey.String(),
		Status:       StatusFailed,
		ObservedAt:   observedAt.UTC(),
		ResetAt:      request.ResetAt.UTC(),
		Remaining:    request.Remaining,
		HasRemaining: request.HasRemaining,
	}
}
