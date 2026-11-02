package management

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"quota-activation/internal/detector"
	"quota-activation/internal/host"
)

type authFilesResponse struct {
	Files []authFileChoice `json:"files"`
}

type authFileChoice struct {
	AuthID       string        `json:"auth_id"`
	Name         string        `json:"name"`
	Label        string        `json:"label"`
	Provider     string        `json:"provider"`
	Disabled     bool          `json:"disabled"`
	Models       []modelChoice `json:"models"`
	QuotaPayload any           `json:"quota_payload"`
}

type modelChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Group string `json:"group,omitempty"`
}

type antigravityCapabilityHints struct {
	ModelIDs []string `json:"model_ids"`
}

type authFileDocument struct {
	AuthID               string                     `json:"auth_id"`
	AuthIDUpper          string                     `json:"authID"`
	ID                   string                     `json:"id"`
	Name                 string                     `json:"name"`
	Account              string                     `json:"account"`
	Email                string                     `json:"email"`
	Provider             string                     `json:"provider"`
	Type                 string                     `json:"type"`
	Disabled             bool                       `json:"disabled"`
	AvailableModels      []string                   `json:"available_models"`
	AvailableModelsUpper []string                   `json:"availableModels"`
	Models               []string                   `json:"models"`
	RecentModels         []string                   `json:"recent_models"`
	AntigravityHints     antigravityCapabilityHints `json:"antigravity_capability_hints"`
}

func (h *Handler) handleAuthFiles(w http.ResponseWriter, request *http.Request) {
	if h.host == nil {
		writeJSON(w, http.StatusOK, authFilesResponse{Files: []authFileChoice{}})
		return
	}
	files, err := h.host.ListAuthFiles(request.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_files_failed", err.Error())
		return
	}
	resolved := h.resolveAuthFiles(request.Context(), files)
	choices := make([]authFileChoice, 0, len(resolved))
	// 同 provider 的模型列表模板可复用，避免每个凭证重复构造。
	modelCache := map[string][]modelChoice{}
	for _, file := range resolved {
		choice, ok := h.authFileChoice(file, modelCache)
		if ok {
			choices = append(choices, choice)
		}
	}
	writeJSON(w, http.StatusOK, authFilesResponse{Files: choices})
}

// resolveAuthFiles 有限并发拉取 runtime，降低凭证很多时 auth-files 接口串行等待。
func (h *Handler) resolveAuthFiles(ctx context.Context, files []host.AuthFile) []host.AuthFile {
	if h.host == nil || len(files) == 0 {
		return files
	}
	resolved := make([]host.AuthFile, len(files))
	type job struct {
		index int
		file  host.AuthFile
	}
	jobs := make(chan job)
	const workers = 8
	var wg sync.WaitGroup
	workerCount := workers
	if len(files) < workerCount {
		workerCount = len(files)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				runtimeFile := item.file
				if item.file.AuthIndex != "" {
					if runtime, err := h.host.GetRuntimeAuthFile(ctx, item.file.AuthIndex); err == nil {
						runtimeFile = mergeRuntimeAuthFile(item.file, runtime)
					}
				}
				resolved[item.index] = runtimeFile
			}
		}()
	}
	for index, file := range files {
		jobs <- job{index: index, file: file}
	}
	close(jobs)
	wg.Wait()
	return resolved
}

func (h *Handler) authFileChoice(file host.AuthFile, modelCache map[string][]modelChoice) (authFileChoice, bool) {
	runtimeFile := file
	document := decodeAuthFile(file)
	authID := firstNonBlank(runtimeFile.ID, file.ID, runtimeFile.Name, file.Name, document.Name, runtimeFile.AuthIndex, file.AuthIndex, document.AuthID, document.AuthIDUpper, document.ID)
	label := firstNonBlank(runtimeFile.Account, runtimeFile.Email, file.Account, file.Email, document.Account, document.Email, document.Name, file.Name, authID)
	provider := normalizeProvider(firstNonBlank(runtimeFile.Provider, runtimeFile.Type, file.Provider, file.Type, document.Provider, document.Type))
	if authID == "" || provider == "" {
		return authFileChoice{}, false
	}
	availableModels := append(runtimeAvailableModels(runtimeFile), document.availableModels()...)
	if detector.Provider(provider) == detector.ProviderAntigravity {
		availableModels = append(availableModels, document.AntigravityHints.ModelIDs...)
		availableModels = uniqueModels(availableModels)
	}
	models := h.modelChoices(provider, availableModels)
	if modelCache != nil {
		cacheKey := provider + "\x00" + strings.Join(availableModels, "\x00")
		if cached, ok := modelCache[cacheKey]; ok {
			models = cached
		} else {
			modelCache[cacheKey] = models
		}
	}
	return authFileChoice{AuthID: authID, Name: firstNonBlank(runtimeFile.Name, file.Name, document.Name), Label: label, Provider: provider, Disabled: runtimeFile.Disabled || file.Disabled || document.Disabled, Models: models, QuotaPayload: h.quotaPayload(file, provider, availableModels, authID)}, true
}

func mergeRuntimeAuthFile(base host.AuthFile, runtime host.AuthFile) host.AuthFile {
	if runtime.AuthIndex == "" {
		runtime.AuthIndex = base.AuthIndex
	}
	if runtime.Name == "" {
		runtime.Name = base.Name
	}
	if runtime.Provider == "" {
		runtime.Provider = base.Provider
	}
	if runtime.Type == "" {
		runtime.Type = base.Type
	}
	if runtime.Email == "" {
		runtime.Email = base.Email
	}
	if runtime.Account == "" {
		runtime.Account = base.Account
	}
	if runtime.Data == nil {
		runtime.Data = base.Data
	}
	return runtime
}

func decodeAuthFile(file host.AuthFile) authFileDocument {
	var document authFileDocument
	if err := json.Unmarshal(file.Data, &document); err != nil {
		return authFileDocument{Name: file.Name}
	}
	return document
}

func normalizeProvider(raw string) string {
	text := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case text == string(detector.ProviderCodex), strings.Contains(text, "codex"):
		return string(detector.ProviderCodex)
	case text == string(detector.ProviderAntigravity), strings.Contains(text, "antigravity"):
		return string(detector.ProviderAntigravity)
	default:
		return ""
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
