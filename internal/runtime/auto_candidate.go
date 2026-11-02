package runtime

import (
	"encoding/json"
	"strings"
	"time"

	"quota-activation/internal/config"
	"quota-activation/internal/detector"
	"quota-activation/internal/host"
	"quota-activation/internal/quotapayload"
	"quota-activation/internal/state"
)

type autoAuthFileDocument struct {
	AuthID               string          `json:"auth_id"`
	AuthIDUpper          string          `json:"authID"`
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Provider             string          `json:"provider"`
	Type                 string          `json:"type"`
	Disabled             bool            `json:"disabled"`
	QuotaPayload         json.RawMessage `json:"quota_payload"`
	QuotaPayloadUpper    json.RawMessage `json:"quotaPayload"`
	AvailableModels      []string        `json:"available_models"`
	AvailableModelsUpper []string        `json:"availableModels"`
	Models               []string        `json:"models"`
	RecentModels         []string        `json:"recent_models"`
}

type autoRuntimeAuthFile struct {
	file host.AuthFile
	ok   bool
}

func (s autoScanSnapshot) autoCandidate(file host.AuthFile, runtimeAuth autoRuntimeAuthFile) (autoCandidate, bool) {
	candidates := s.autoCandidates(file, runtimeAuth)
	if len(candidates) == 0 {
		return autoCandidate{}, false
	}
	return candidates[0], true
}

func (s autoScanSnapshot) autoCandidates(file host.AuthFile, runtimeAuth autoRuntimeAuthFile) []autoCandidate {
	document := decodeAutoAuthFile(file)
	// 宿主 scheduler 候选 Auth.ID 与 host.auth.list 的 id/name（文件名）一致；auth_index 哈希不得优先，否则 pick 命中失败。
	authID := firstNonBlankAuto(file.ID, file.Name, document.Name, document.ID, document.AuthID, document.AuthIDUpper, file.AuthIndex)
	provider, ok := autoProvider(firstNonBlankAuto(file.Provider, file.Type, document.Provider, document.Type))
	if !ok || authID == "" {
		return nil
	}
	disabled := file.Disabled || document.Disabled
	if disabled && !s.config.EnableBeforeActivation {
		return nil
	}
	available := autoCandidateModels(file, document, runtimeAuth)
	observedAt := s.now().UTC()
	if provider == detector.ProviderAntigravity {
		pairs := autoAntigravityModels(s.config, available)
		out := make([]autoCandidate, 0, len(pairs))
		for _, pair := range pairs {
			payload, ok := autoCandidatePayloadWithGroup(file, document, runtimeAuth, provider, pair.Model, authID, s.store, observedAt, s.host, string(antigravityPairModelGroup(pair)))
			if !ok {
				continue
			}
			out = append(out, autoCandidate{
				authID:     authID,
				authIndex:  strings.TrimSpace(file.AuthIndex),
				provider:   provider,
				model:      pair.Model,
				modelGroup: antigravityPairModelGroup(pair),
				disabled:   disabled,
				payload:    payload,
			})
		}
		return out
	}
	model, ok := autoModel(s.config, provider, available)
	if !ok {
		return nil
	}
	payload, ok := autoCandidatePayload(file, document, runtimeAuth, provider, model, authID, s.store, observedAt, s.host)
	if !ok {
		return nil
	}
	return []autoCandidate{{
		authID:    authID,
		authIndex: strings.TrimSpace(file.AuthIndex),
		provider:  provider,
		model:     model,
		disabled:  disabled,
		payload:   payload,
	}}
}

func autoCandidatePayload(file host.AuthFile, document autoAuthFileDocument, runtimeAuth autoRuntimeAuthFile, provider detector.Provider, model string, authID string, store *state.Store, observedAt time.Time, hostClient host.Client) ([]byte, bool) {
	return autoCandidatePayloadWithGroup(file, document, runtimeAuth, provider, model, authID, store, observedAt, hostClient, autoPayloadModelGroup(provider, model))
}

func autoCandidatePayloadWithGroup(file host.AuthFile, document autoAuthFileDocument, runtimeAuth autoRuntimeAuthFile, provider detector.Provider, model string, authID string, store *state.Store, observedAt time.Time, hostClient host.Client, modelGroup string) ([]byte, bool) {
	if runtimeAuth.ok {
		runtimeDocument := decodeAutoAuthFile(runtimeAuth.file)
		if payload, ok := autoQuotaPayload(runtimeAuth.file, runtimeDocument); ok && keepTrueAutoPayload(provider, payload, observedAt) {
			return payload, true
		}
	}
	if payload, ok := autoQuotaPayload(file, document); ok && keepTrueAutoPayload(provider, payload, observedAt) {
		return payload, true
	}
	if store != nil {
		record, ok := latestSuccessForAutoPayload(store, authID, provider, modelGroup)
		if ok {
			if payload, ok := syntheticInferredQuotaPayload(provider, model, record, observedAt); ok {
				return payload, true
			}
		}
	}
	if provider == detector.ProviderCodex {
		if payload, ok := syntheticCodexPlanPayload(file, model, observedAt, hostClient); ok {
			return payload, true
		}
	}
	return syntheticAutoQuotaPayload(provider, model, observedAt)
}

func autoCandidateModels(file host.AuthFile, document autoAuthFileDocument, runtimeAuth autoRuntimeAuthFile) []string {
	models := make([]string, 0)
	if runtimeAuth.ok {
		runtimeDocument := decodeAutoAuthFile(runtimeAuth.file)
		models = append(models, autoRuntimeModels(runtimeAuth.file)...)
		models = append(models, runtimeDocument.availableModels()...)
	}
	models = append(models, autoRuntimeModels(file)...)
	models = append(models, document.availableModels()...)
	return models
}

func decodeAutoAuthFile(file host.AuthFile) autoAuthFileDocument {
	var document autoAuthFileDocument
	if err := json.Unmarshal(file.Data, &document); err != nil {
		return autoAuthFileDocument{Name: file.Name}
	}
	return document
}

func autoProvider(raw string) (detector.Provider, bool) {
	text := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case text == string(detector.ProviderCodex), strings.Contains(text, "codex"):
		return detector.ProviderCodex, true
	case text == string(detector.ProviderAntigravity), strings.Contains(text, "antigravity"):
		return detector.ProviderAntigravity, true
	default:
		return detector.ProviderUnknown, false
	}
}

func autoModel(cfg config.Config, provider detector.Provider, availableModels []string) (string, bool) {
	switch provider {
	case detector.ProviderCodex:
		model := strings.TrimSpace(cfg.ActivationModels.Codex.Models)
		if model == "" {
			return "", false
		}
		// CPA host.auth.list/get_runtime 通常不返回 models；有列表时做存在性校验，无列表时直接使用配置模型。
		if len(availableModels) > 0 && !hasAutoModel(availableModels, model) {
			return "", false
		}
		return model, true
	case detector.ProviderAntigravity:
		return autoAntigravityModel(cfg, availableModels)
	case detector.ProviderUnknown:
		return "", false
	default:
		return "", false
	}
}

func latestSuccessForAutoPayload(store *state.Store, authID string, provider detector.Provider, modelGroup string) (state.Record, bool) {
	if store == nil {
		return state.Record{}, false
	}
	if provider == detector.ProviderCodex {
		return store.UsableLatestSuccess(authID, string(provider))
	}
	if provider == detector.ProviderAntigravity {
		return store.LatestSuccessForModelGroup(authID, string(provider), modelGroup)
	}
	return store.LatestSuccess(authID, string(provider))
}

func autoPayloadModelGroup(provider detector.Provider, model string) string {
	if provider != detector.ProviderAntigravity {
		return ""
	}
	inferred, ok := antigravityConfigModelGroup(model)
	if !ok {
		return ""
	}
	return string(inferred)
}

func syntheticInferredQuotaPayload(provider detector.Provider, model string, record state.Record, observedAt time.Time) ([]byte, bool) {
	// 幂等：LatestSuccess 的 ResetAt 仍晚于 now 时，用该记录 Window+ResetAt 合成，
	// 保证 CycleKey 与上次 success 一致 → detector 判「额度周期已处理」。
	// 仅当无 success 或 ResetAt 已过期时，才回退 5h 短窗允许新周期。
	// 删除「Codex 无 remaining 就禁止 inferred」导致的 CycleKey 漂移。
	windowName, limitSeconds, resetAt, ok := quotapayload.WindowFromSuccess(record, observedAt)
	if !ok {
		return nil, false
	}
	return marshalAutoQuotaPayload(provider, model, windowName, limitSeconds, resetAt)
}

// recordHasRemainingSnapshot 判断 success 是否带可恢复判定的 remaining 证据。
func recordHasRemainingSnapshot(record state.Record) bool {
	if record.HasRemaining {
		return true
	}
	// omitempty：仅有 Remaining!=0 也视作已写入 remaining。
	return record.Remaining != 0
}

func syntheticAutoQuotaPayload(provider detector.Provider, model string, observedAt time.Time) ([]byte, bool) {
	windowName, limitSeconds, resetAt, ok := quotapayload.SyntheticWindow(provider, observedAt)
	if !ok {
		return nil, false
	}
	return marshalAutoQuotaPayload(provider, model, windowName, limitSeconds, resetAt)
}

func marshalAutoQuotaPayload(provider detector.Provider, model string, windowName string, limitSeconds int, resetAt time.Time) ([]byte, bool) {
	return quotapayload.Marshal(provider, []string{model}, windowName, limitSeconds, resetAt)
}

func firstNonBlankAuto(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
