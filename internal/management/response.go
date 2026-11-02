package management

import (
	"encoding/json"
	"net/http"
	"time"

	"quota-activation/internal/activator"
	"quota-activation/internal/state"
)

type activationResponse struct {
	AuthID           string            `json:"auth_id"`
	Provider         string            `json:"provider"`
	Window           string            `json:"window"`
	CycleKey         string            `json:"cycle_key"`
	Status           string            `json:"status"`
	Success          bool              `json:"success"`
	Nonce            string            `json:"nonce,omitempty"`
	HTTPStatus       int               `json:"http_status,omitempty"`
	TemporaryEnabled bool              `json:"temporary_enabled,omitempty"`
	RestoredDisabled bool              `json:"restored_disabled,omitempty"`
	ObservedAt       time.Time         `json:"observed_at"`
	ResetAt          time.Time         `json:"reset_at"`
	LastError        string            `json:"last_error,omitempty"`
	WakePath         string            `json:"wake_path,omitempty"`
	StateRecords     []state.Record    `json:"records,omitempty"`
	LatestStatus     string            `json:"latest_status,omitempty"`
	Routes           []Route           `json:"routes,omitempty"`
	Resources        []Resource        `json:"resources,omitempty"`
	Config           *diagnosticConfig `json:"config,omitempty"`
	RunHistory       any               `json:"run_history,omitempty"`
}

type diagnosticConfig struct {
	AutoActivate           bool   `json:"auto_activate"`
	EnableBeforeActivation bool   `json:"enable_before_activation"`
	ActivationModelCodex   string `json:"activation_model_codex"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func responseFromResult(result activator.Result) activationResponse {
	return activationResponse{
		AuthID:           state.Redact(result.AuthID),
		Provider:         state.Redact(result.Provider),
		Window:           state.Redact(result.Window),
		CycleKey:         state.Redact(result.CycleKey),
		Status:           string(result.Status),
		Success:          result.Success,
		HTTPStatus:       result.HTTPStatus,
		TemporaryEnabled: result.TemporaryEnabled,
		RestoredDisabled: result.RestoredDisabled,
		ObservedAt:       result.ObservedAt.UTC(),
		ResetAt:          result.ResetAt.UTC(),
		LastError:        activator.LocalizeUserMessage(state.Redact(result.LastError)),
		WakePath:         string(result.WakePath),
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: state.Redact(message)})
}
