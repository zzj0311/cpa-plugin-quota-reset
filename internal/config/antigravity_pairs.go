package config

import "strings"

const maxAntigravityModelPairs = 3

// AntigravityModelPair 是一组已配对的 Antigravity 模型组与模型名。
type AntigravityModelPair struct {
	Group string
	Model string
}

// AntigravityActivationModels 定义 Antigravity 不同模型组的唤醒模型。
type AntigravityActivationModels struct {
	ModelsGroup string
	Models      string
	Pairs       []AntigravityModelPair
}

type rawAntigravityModelsGroup struct {
	ModelsGroup     *string `json:"models_group"`
	Models          *string `json:"models"`
	Gemini          *string `json:"gemini"`
	ClaudeGPT       *string `json:"claude_gpt"`
	EnableGemini    *bool   `json:"enable_gemini"`
	EnableClaudeGPT *bool   `json:"enable_claude_gpt"`
}

type legacyAntigravityModels struct {
	gemini          string
	claudeGPT       string
	enableGemini    bool
	enableClaudeGPT bool
}

func (raw rawAntigravityModelsGroup) parse(base AntigravityActivationModels) (AntigravityActivationModels, error) {
	models := base
	if raw.ModelsGroup != nil || raw.Models != nil {
		if raw.ModelsGroup != nil {
			models.ModelsGroup = strings.TrimSpace(*raw.ModelsGroup)
		}
		if raw.Models != nil {
			models.Models = strings.TrimSpace(*raw.Models)
		}
		return parseAntigravityModelPairs(models)
	}

	legacy := legacyAntigravityModels{}
	if raw.Gemini != nil {
		legacy.gemini = strings.TrimSpace(*raw.Gemini)
	}
	if raw.ClaudeGPT != nil {
		legacy.claudeGPT = strings.TrimSpace(*raw.ClaudeGPT)
	}
	if raw.EnableGemini != nil {
		legacy.enableGemini = *raw.EnableGemini
	}
	if raw.EnableClaudeGPT != nil {
		legacy.enableClaudeGPT = *raw.EnableClaudeGPT
	}
	if legacy.enableGemini && legacy.enableClaudeGPT {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity", "Gemini 与 Claude/GPT 只能启用一种")
	}
	if legacy.enableGemini && legacy.gemini == "" {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity.gemini", "启用 Gemini 组时必须填写")
	}
	if legacy.enableClaudeGPT && legacy.claudeGPT == "" {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity.claude_gpt", "启用 Claude/GPT 组时必须填写")
	}
	if legacy.enableGemini {
		models.ModelsGroup = "gemini"
		models.Models = legacy.gemini
	}
	if legacy.enableClaudeGPT {
		models.ModelsGroup = "claude_gpt"
		models.Models = legacy.claudeGPT
	}
	return parseAntigravityModelPairs(models)
}

func parseAntigravityModelPairs(models AntigravityActivationModels) (AntigravityActivationModels, error) {
	groupText := strings.TrimSpace(models.ModelsGroup)
	modelText := strings.TrimSpace(models.Models)
	if groupText == "" && modelText == "" {
		models.ModelsGroup = ""
		models.Models = ""
		models.Pairs = nil
		return models, nil
	}
	if strings.Contains(groupText, ",") || strings.Contains(modelText, ",") {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity", "每个模型组只能填写一个模型")
	}
	if groupText == "" {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity.models_group", "填写模型名时必须选择模型组")
	}
	if modelText == "" {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity.models", "选择模型组时必须填写")
	}

	groups, err := splitAntigravitySegments(groupText)
	if err != nil {
		return AntigravityActivationModels{}, err
	}
	modelNames, err := splitAntigravitySegments(modelText)
	if err != nil {
		return AntigravityActivationModels{}, err
	}
	if len(groups) != len(modelNames) {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity", "模型组与模型数量必须一致")
	}
	if len(groups) > maxAntigravityModelPairs {
		return AntigravityActivationModels{}, invalid("activation_models.antigravity", "最多配置 3 个模型组")
	}

	seen := make(map[string]struct{}, len(groups))
	pairs := make([]AntigravityModelPair, 0, len(groups))
	for i, group := range groups {
		if err := validateAntigravityGroup(group); err != nil {
			return AntigravityActivationModels{}, err
		}
		if _, exists := seen[group]; exists {
			return AntigravityActivationModels{}, invalid("activation_models.antigravity", "模型组不能重复")
		}
		seen[group] = struct{}{}
		modelName := modelNames[i]
		if err := validateAntigravityModelMatchesGroup(group, modelName); err != nil {
			return AntigravityActivationModels{}, err
		}
		pairs = append(pairs, AntigravityModelPair{Group: group, Model: modelName})
	}

	models.Pairs = pairs
	models.ModelsGroup = groupText
	models.Models = modelText
	return models, nil
}

func splitAntigravitySegments(raw string) ([]string, error) {
	parts := strings.Split(raw, "|")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, invalid("activation_models.antigravity", "不能包含空的模型组或模型")
		}
		segments = append(segments, trimmed)
	}
	return segments, nil
}

func validateAntigravityGroup(group string) error {
	switch group {
	case "gemini", "claude_gpt", "gemini_4_argon":
		return nil
	default:
		return invalid("activation_models.antigravity.models_group", "必须是 gemini、claude_gpt 或 gemini_4_argon")
	}
}

func validateAntigravityModelMatchesGroup(group string, model string) error {
	inferred, ok := inferConfiguredAntigravityGroup(model)
	if ok && inferred != group {
		return invalid("activation_models.antigravity.models", "必须与所选模型组一致")
	}
	return nil
}

func inferConfiguredAntigravityGroup(model string) (string, bool) {
	text := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(text, "claude") || strings.Contains(text, "gpt") || strings.Contains(text, "openai") {
		return "claude_gpt", true
	}
	if containsGemini4ArgonToken(text) {
		return "gemini_4_argon", true
	}
	if strings.Contains(text, "gemini") {
		return "gemini", true
	}
	return "", false
}

func containsGemini4ArgonToken(text string) bool {
	normalized := strings.ToLower(text)
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.Join(strings.Fields(normalized), " ")
	return strings.Contains(normalized, "gemini 4 argon")
}
