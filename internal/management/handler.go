package management

import (
	"context"
	"errors"
	"net/http"

	"quota-activation/internal/activator"
	"quota-activation/internal/detector"
	"quota-activation/internal/host"
	"quota-activation/internal/livequota"
	"quota-activation/internal/reconcile"
	"quota-activation/internal/state"
)

// ServeHTTP 分发管理 API 请求和插件资源页请求。
func (h *Handler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == resourceStatusPath:
		h.handlePage(w)
	case request.Method == http.MethodGet && (request.URL.Path == managementPrefix || request.URL.Path == managementPrefix+"/status"):
		h.handleStatus(w)
	case request.Method == http.MethodGet && request.URL.Path == managementPrefix+"/auth-files":
		h.handleAuthFiles(w, request)
	case request.Method == http.MethodPost && request.URL.Path == managementPrefix+"/activate":
		h.handleActivate(w, request)
	case request.Method == http.MethodGet && request.URL.Path == managementPrefix+"/diagnostics":
		h.handleDiagnostics(w)
	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
	}
}

func (h *Handler) handleActivate(w http.ResponseWriter, request *http.Request) {
	if request.Context().Err() != nil {
		writeError(w, http.StatusRequestTimeout, "request_canceled", "request canceled")
		return
	}
	decoded, err := decodeActivationRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	activation, shouldActivate, err := h.toActivatorRequest(decoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx := request.Context()
	lookup := []string{activation.AuthID}
	if idx, resolveErr := livequota.ResolveAuthIndex(ctx, h.host, lookup); resolveErr == nil && idx != "" {
		lookup = []string{idx, activation.AuthID}
	}
	bearer := reconcile.BearerFromRequest(request.Header.Get("Authorization"))
	localBlock := reconcile.RuntimeLocalBlock(ctx, h.host, lookup)

	probe, probeErr := reconcile.CodexGate(ctx, h.host, h.config, activation.Provider, lookup)
	if activation.Provider == detector.ProviderCodex && probeErr != nil {
		failed := activator.Result{
			AuthID:     activation.AuthID,
			Provider:   string(activation.Provider),
			Window:     string(activation.Window),
			CycleKey:   activation.CycleKey.String(),
			Status:     activator.StatusFailed,
			Success:    false,
			ObservedAt: activation.ObservedAt.UTC(),
			ResetAt:    activation.ResetAt.UTC(),
			LastError:  "上游额度探测失败：" + activator.LocalizeUserMessage(state.Redact(probeErr.Error())),
		}
		if h.onActivation != nil {
			h.onActivation(failed, probeErr)
		}
		response := responseFromResult(failed)
		h.storeLatest(response)
		writeJSON(w, http.StatusBadGateway, response)
		return
	}

	if !shouldActivate {
		// 周期已处理：若 live 探针确认恢复且本地仍冷却，则清除本地冷却。
		skipped := activator.Result{
			AuthID:       activation.AuthID,
			Provider:     string(activation.Provider),
			Window:       string(activation.Window),
			CycleKey:     activation.CycleKey.String(),
			Status:       activator.StatusSkipped,
			Success:      false,
			ObservedAt:   activation.ObservedAt.UTC(),
			ResetAt:      activation.ResetAt.UTC(),
			LastError:    "本周期已唤醒，等待重置",
			Remaining:    activation.Remaining,
			HasRemaining: activation.HasRemaining,
		}
		if probe.Recovered {
			if cleared, clearErr := reconcile.ClearIfNeeded(ctx, h.host, h.config, lookup, bearer, true, localBlock, false); clearErr != nil {
				skipped.LastError = "本周期已唤醒；清除本地冷却失败：" + activator.LocalizeUserMessage(state.Redact(clearErr.Error()))
			} else if cleared {
				skipped.LastError = "本周期已唤醒；已清除本地冷却"
				skipped.Warning = "local_cooldown_cleared"
			} else if !localBlock {
				skipped.LastError = "本周期已唤醒，本地无冷却"
			}
		} else if activation.Provider == detector.ProviderCodex {
			skipped.LastError = "本周期已唤醒，但上游额度未恢复（used_percent≥阈值）"
		}
		if h.onActivation != nil {
			h.onActivation(skipped, nil)
		}
		response := responseFromResult(skipped)
		h.storeLatest(response)
		writeJSON(w, http.StatusOK, response)
		return
	}

	if activation.Provider == detector.ProviderCodex && !probe.Recovered {
		skipped := activator.Result{
			AuthID:     activation.AuthID,
			Provider:   string(activation.Provider),
			Window:     string(activation.Window),
			CycleKey:   activation.CycleKey.String(),
			Status:     activator.StatusSkipped,
			Success:    false,
			ObservedAt: activation.ObservedAt.UTC(),
			ResetAt:    activation.ResetAt.UTC(),
			LastError:  "上游额度未恢复，跳过唤醒",
		}
		if h.onActivation != nil {
			h.onActivation(skipped, nil)
		}
		response := responseFromResult(skipped)
		h.storeLatest(response)
		writeJSON(w, http.StatusOK, response)
		return
	}

	result, err := h.activator.Activate(ctx, activation)
	wakeOK := err == nil && result.Status == activator.StatusSuccess && result.Success
	if wakeOK || (err == nil && probe.Recovered && localBlock) {
		if cleared, clearErr := reconcile.ClearIfNeeded(ctx, h.host, h.config, lookup, bearer, probe.Recovered, localBlock, wakeOK); clearErr != nil {
			if result.LastError == "" {
				result.LastError = "清除本地冷却失败：" + activator.LocalizeUserMessage(state.Redact(clearErr.Error()))
			}
			if result.Warning == "" {
				result.Warning = clearErr.Error()
			}
		} else if cleared {
			if result.Warning == "" {
				result.Warning = "local_cooldown_cleared"
			}
		}
	}
	if h.onActivation != nil {
		h.onActivation(result, err)
	}
	response := responseFromResult(result)
	if err != nil {
		// 管理面 LastError 必须纯中文（与历史/状态一致）。
		response.LastError = activator.LocalizeUserMessage(state.Redact(err.Error()))
		h.storeLatest(response)
		writeJSON(w, statusForActivationError(err), response)
		return
	}
	h.storeLatest(response)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) handleStatus(w http.ResponseWriter) {
	response := h.latestResponse()
	response.LatestStatus = latestStatus(response.Status)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) handleDiagnostics(w http.ResponseWriter) {
	response := h.latestResponse()
	response.LatestStatus = latestStatus(response.Status)
	response.Routes = Register().Routes
	response.Resources = Register().Resources
	response.Config = &diagnosticConfig{
		AutoActivate:           h.config.AutoActivate,
		EnableBeforeActivation: h.config.EnableBeforeActivation,
		ActivationModelCodex:   state.Redact(h.config.ActivationModels.Codex.Models),
	}
	if response.AuthID != "" {
		response.StateRecords = []state.Record{{
			AuthID:     response.AuthID,
			Provider:   response.Provider,
			Window:     response.Window,
			CycleKey:   response.CycleKey,
			ObservedAt: response.ObservedAt,
			ResetAt:    response.ResetAt,
			LastResult: response.Status,
			LastError:  response.LastError,
		}}
	}
	if h.runHistory != nil {
		if history := h.runHistory(); history != nil {
			response.RunHistory = history
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) storeLatest(response activationResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.latest = response
}

func (h *Handler) latestResponse() activationResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.latest
}

func latestStatus(status string) string {
	if status == "" {
		return "idle"
	}
	return state.Redact(status)
}

func statusForActivationError(err error) int {
	switch {
	case errors.Is(err, activator.ErrBusy), errors.Is(err, activator.ErrDisabledCredential):
		return http.StatusConflict
	case errors.Is(err, host.ErrModelExecuteStatus):
		return http.StatusBadGateway
	case errors.Is(err, activator.ErrNetworkFailure), activator.IsNetworkFailure(err):
		return http.StatusBadGateway
	case errors.Is(err, activator.ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}
