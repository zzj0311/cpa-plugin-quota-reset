package management

import (
	"fmt"
	"strings"

	"quota-activation/internal/detector"
	"quota-activation/internal/host"
)

func (d authFileDocument) availableModels() []string {
	models := append([]string(nil), d.AvailableModels...)
	models = append(models, d.AvailableModelsUpper...)
	models = append(models, d.Models...)
	models = append(models, d.RecentModels...)
	return uniqueModels(models)
}

func runtimeAvailableModels(file host.AuthFile) []string {
	models := append([]string(nil), file.Models...)
	models = append(models, file.RecentModels...)
	models = append(models, stringSliceFromMap(file.Metadata, "available_models")...)
	models = append(models, stringSliceFromMap(file.Metadata, "availableModels")...)
	models = append(models, stringSliceFromMap(file.Metadata, "models")...)
	models = append(models, stringSliceFromMap(file.Attributes, "available_models")...)
	models = append(models, stringSliceFromMap(file.Attributes, "availableModels")...)
	models = append(models, stringSliceFromMap(file.Attributes, "models")...)
	return uniqueModels(models)
}

func uniqueModels(models []string) []string {
	choices := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		choices = append(choices, trimmed)
	}
	return choices
}

func stringSliceFromMap(values map[string]any, key string) []string {
	if len(values) == 0 {
		return nil
	}
	switch raw := values[key].(type) {
	case []string:
		return raw
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if value, ok := item.(string); ok {
				out = append(out, value)
			}
		}
		return out
	case string:
		return strings.Split(raw, ",")
	default:
		return nil
	}
}

func (h *Handler) modelChoices(provider string, availableModels []string) []modelChoice {
	switch detector.Provider(provider) {
	case detector.ProviderCodex:
		choices := make([]modelChoice, 0, len(availableModels))
		for _, model := range availableModels {
			choices = append(choices, modelChoice{Value: model, Label: fmt.Sprintf("Codex · %s", model)})
		}
		return choices
	case detector.ProviderAntigravity:
		choices := make([]modelChoice, 0, len(availableModels))
		for _, model := range availableModels {
			group := antigravityModelGroup(model)
			if group == detector.ModelGroupNone {
				continue
			}
			choices = append(choices, modelChoice{Value: model, Label: fmt.Sprintf("%s · %s", antigravityModelGroupLabel(group), model), Group: string(group)})
		}
		return choices
	default:
		return nil
	}
}

func antigravityModelGroup(model string) detector.ModelGroup {
	group, ok := detector.InferModelGroup(model)
	if !ok {
		return detector.ModelGroupNone
	}
	return group
}

func antigravityModelGroupLabel(group detector.ModelGroup) string {
	switch group {
	case detector.ModelGroupGemini:
		return "Gemini"
	case detector.ModelGroupGemini4Argon:
		return "Gemini 4 Argon"
	default:
		return "Claude/GPT"
	}
}
