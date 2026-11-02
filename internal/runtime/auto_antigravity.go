package runtime

import (
	"strings"

	"quota-activation/internal/config"
	"quota-activation/internal/detector"
)

func autoAntigravityModels(cfg config.Config, availableModels []string) []config.AntigravityModelPair {
	pairs := cfg.ActivationModels.Antigravity.Pairs
	if len(pairs) == 0 {
		model, ok := autoAntigravityModel(cfg, availableModels)
		if !ok {
			return nil
		}
		return []config.AntigravityModelPair{{
			Group: strings.TrimSpace(cfg.ActivationModels.Antigravity.ModelsGroup),
			Model: model,
		}}
	}
	out := make([]config.AntigravityModelPair, 0, len(pairs))
	for _, pair := range pairs {
		if !antigravityPairEligible(pair, availableModels) {
			continue
		}
		out = append(out, config.AntigravityModelPair{
			Group: strings.TrimSpace(pair.Group),
			Model: strings.TrimSpace(pair.Model),
		})
	}
	return out
}

func autoAntigravityModel(cfg config.Config, availableModels []string) (string, bool) {
	model := strings.TrimSpace(cfg.ActivationModels.Antigravity.Models)
	if model == "" {
		return "", false
	}
	pair := config.AntigravityModelPair{
		Group: strings.TrimSpace(cfg.ActivationModels.Antigravity.ModelsGroup),
		Model: model,
	}
	if !antigravityPairEligible(pair, availableModels) {
		return "", false
	}
	return model, true
}

func antigravityPairEligible(pair config.AntigravityModelPair, availableModels []string) bool {
	model := strings.TrimSpace(pair.Model)
	if model == "" {
		return false
	}
	group := strings.TrimSpace(pair.Group)
	if group != "" {
		inferred, ok := antigravityConfigModelGroup(model)
		if ok && string(inferred) != group {
			return false
		}
	}
	if len(availableModels) > 0 && !hasAutoModel(availableModels, model) {
		return false
	}
	return true
}

func antigravityPairModelGroup(pair config.AntigravityModelPair) detector.ModelGroup {
	group := strings.TrimSpace(pair.Group)
	if group != "" {
		return detector.ModelGroup(group)
	}
	inferred, ok := antigravityConfigModelGroup(pair.Model)
	if !ok {
		return detector.ModelGroupNone
	}
	return inferred
}

func antigravityConfigModelGroup(model string) (detector.ModelGroup, bool) {
	return detector.InferModelGroup(model)
}
