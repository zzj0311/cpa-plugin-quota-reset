package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrInvalidConfig 标识插件配置解析或校验失败。
	ErrInvalidConfig = errors.New("配置无效")
)

// ActivationTransport 表示配额唤醒请求的宿主传输方式。
type ActivationTransport string

const (
	// ActivationTransportDirectHTTP 通过 host.http.do 直接发送唤醒请求。
	ActivationTransportDirectHTTP ActivationTransport = "direct_http"
	// ActivationTransportSchedulerBoost 通过调度器提升优先级后走宿主模型路径。
	ActivationTransportSchedulerBoost ActivationTransport = "scheduler_boost"
)

// Config 是 quota-activation 插件配置解析后的稳定形态。
type Config struct {
	AutoActivate             bool
	ScanInterval             time.Duration
	ActivationRequestTimeout time.Duration
	MaxConcurrency           int
	ActivationPrompt         string
	StatePath                string
	EnableBeforeActivation   bool
	// ActivationTransport 默认 direct_http；允许 scheduler_boost。
	ActivationTransport ActivationTransport
	// SchedulerBoostFallback：direct_http 遇传输/宿主类失败时是否回退 legacy scheduler_boost；默认 true。
	// 业务/严格成功判定失败禁止回退（不得伪装成功）。
	SchedulerBoostFallback bool
	ActivationModels       ActivationModels
	// RequireLiveQuotaForAuto：自动路径必须先通过 live Codex usage 探针。
	RequireLiveQuotaForAuto bool
	// ClearLocalCooldown：在确认额度恢复后调用 reset-quota 清除 CPA 本地冷却。
	ClearLocalCooldown bool
	// ManagementLoopbackURL：CPA 进程内 reset-quota 地址。
	ManagementLoopbackURL string
	// ManagementKey：自动路径调用 reset-quota 时使用（手动路径复用请求 Authorization）。
	ManagementKey string
	// CodexUsageURL：live 探针 URL。
	CodexUsageURL string
	// CodexUsedPercentThreshold：used_percent 低于该值才算恢复（默认 95）。
	CodexUsedPercentThreshold float64
}

// ActivationModels 定义不同 provider 唤醒请求必须显式配置的模型。
type ActivationModels struct {
	Codex       CodexActivationModels
	Antigravity AntigravityActivationModels
}

// CodexActivationModels 定义 Codex 自动唤醒模型。
type CodexActivationModels struct {
	Models string
}

type rawConfig struct {
	Enabled                   *bool                `json:"enabled"`
	Priority                  *int                 `json:"priority"`
	AutoActivate              *bool                `json:"auto_activate"`
	ScanInterval              *string              `json:"scan_interval"`
	ActivationRequestTimeout  *string              `json:"activation_request_timeout"`
	MaxConcurrency            *int                 `json:"max_concurrency"`
	ActivationPrompt          *string              `json:"activation_prompt"`
	StatePath                 *string              `json:"state_path"`
	EnableBeforeActivation    *bool                `json:"enable_before_activation"`
	ActivationTransport       *string              `json:"activation_transport"`
	SchedulerBoostFallback    *bool                `json:"scheduler_boost_fallback"`
	ActivationModels          *rawActivationModels `json:"activation_models"`
	RequireLiveQuotaForAuto   *bool                `json:"require_live_quota_for_auto"`
	ClearLocalCooldown        *bool                `json:"clear_local_cooldown"`
	ManagementLoopbackURL     *string              `json:"management_loopback_url"`
	ManagementKey             *string              `json:"management_key"`
	CodexUsageURL             *string              `json:"codex_usage_url"`
	CodexUsedPercentThreshold *float64             `json:"codex_used_percent_threshold"`
}

type rawActivationModels struct {
	Codex       *rawCodexActivationModels  `json:"codex"`
	Antigravity *rawAntigravityModelsGroup `json:"antigravity"`
}

type rawCodexActivationModels struct {
	Models *string `json:"models"`
}

// Default 返回稳定默认配置。
// AutoActivate 默认 false（README 强制：手动唤醒为默认）；EnableBeforeActivation 默认 true，
// 使 disabled Free 可进入 auto 候选并由 activator 启用；Codex 默认模型 gpt-5-mini。
func Default() Config {
	return Config{
		AutoActivate:             false,
		ScanInterval:             30 * time.Minute,
		ActivationRequestTimeout: time.Minute,
		MaxConcurrency:           1,
		ActivationPrompt:         "quota activation ping",
		StatePath:                "quota-reset/state.json",
		EnableBeforeActivation:   true,
		ActivationTransport:      ActivationTransportDirectHTTP,
		SchedulerBoostFallback:   true,
		RequireLiveQuotaForAuto:  true,
		ClearLocalCooldown:        true,
		ManagementLoopbackURL:    "http://127.0.0.1:8317/v0/management/reset-quota",
		CodexUsageURL:            "https://chatgpt.com/backend-api/wham/usage",
		CodexUsedPercentThreshold: 95,
		ActivationModels: ActivationModels{
			Codex: CodexActivationModels{Models: "gpt-5-mini"},
		},
	}
}

// UnmarshalJSON 兼容旧版 activation_models.codex 标量与新版对象结构。
func (raw *rawCodexActivationModels) UnmarshalJSON(data []byte) error {
	var legacy string
	if err := json.Unmarshal(data, &legacy); err == nil {
		trimmed := strings.TrimSpace(legacy)
		raw.Models = &trimmed
		return nil
	}
	type rawCodexAlias rawCodexActivationModels
	var decoded rawCodexAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*raw = rawCodexActivationModels(decoded)
	return nil
}

// Parse 将 CPA 传入的插件配置 JSON 字节解析为已校验配置。
func Parse(data []byte) (Config, error) {
	raw, err := decodeRaw(data)
	if err != nil {
		return Config{}, fmt.Errorf("解析配置: %w", err)
	}
	cfg, err := raw.apply(Default())
	if err != nil {
		return Config{}, fmt.Errorf("校验配置: %w", err)
	}
	return cfg, nil
}

func decodeRaw(data []byte) (rawConfig, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return rawConfig{}, nil
	}
	var raw rawConfig
	if trimmed[0] == '{' {
		decoder := json.NewDecoder(strings.NewReader(trimmed))
		if err := decoder.Decode(&raw); err != nil {
			return rawConfig{}, invalid("config", fmt.Sprintf("必须匹配配置结构: %v", err))
		}
		return raw, nil
	}

	yamlMap, err := parseYAMLMap(trimmed)
	if err != nil {
		return rawConfig{}, err
	}
	encoded, err := json.Marshal(yamlMap)
	if err != nil {
		return rawConfig{}, invalid("config", "YAML 编码错误")
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	if err := decoder.Decode(&raw); err != nil {
		return rawConfig{}, invalid("config", fmt.Sprintf("YAML 不匹配配置结构: %v", err))
	}
	return raw, nil
}

func (raw rawConfig) apply(cfg Config) (Config, error) {
	if raw.AutoActivate != nil {
		cfg.AutoActivate = *raw.AutoActivate
	}
	if raw.ActivationPrompt != nil && strings.TrimSpace(*raw.ActivationPrompt) != "" {
		cfg.ActivationPrompt = *raw.ActivationPrompt
	}
	if raw.StatePath != nil {
		cfg.StatePath = *raw.StatePath
	}
	if raw.EnableBeforeActivation != nil {
		cfg.EnableBeforeActivation = *raw.EnableBeforeActivation
	}
	if raw.SchedulerBoostFallback != nil {
		cfg.SchedulerBoostFallback = *raw.SchedulerBoostFallback
	}
	if raw.RequireLiveQuotaForAuto != nil {
		cfg.RequireLiveQuotaForAuto = *raw.RequireLiveQuotaForAuto
	}
	if raw.ClearLocalCooldown != nil {
		cfg.ClearLocalCooldown = *raw.ClearLocalCooldown
	}
	if raw.ManagementLoopbackURL != nil && strings.TrimSpace(*raw.ManagementLoopbackURL) != "" {
		cfg.ManagementLoopbackURL = strings.TrimSpace(*raw.ManagementLoopbackURL)
	}
	if raw.ManagementKey != nil {
		cfg.ManagementKey = strings.TrimSpace(*raw.ManagementKey)
	}
	if raw.CodexUsageURL != nil && strings.TrimSpace(*raw.CodexUsageURL) != "" {
		cfg.CodexUsageURL = strings.TrimSpace(*raw.CodexUsageURL)
	}
	if raw.CodexUsedPercentThreshold != nil {
		if *raw.CodexUsedPercentThreshold <= 0 || *raw.CodexUsedPercentThreshold > 100 {
			return Config{}, invalid("codex_used_percent_threshold", "必须在 (0, 100] 内")
		}
		cfg.CodexUsedPercentThreshold = *raw.CodexUsedPercentThreshold
	}
	if raw.ActivationTransport != nil {
		transport, err := parseActivationTransport(*raw.ActivationTransport)
		if err != nil {
			return Config{}, err
		}
		cfg.ActivationTransport = transport
	}
	if raw.MaxConcurrency != nil {
		cfg.MaxConcurrency = *raw.MaxConcurrency
	}
	if cfg.MaxConcurrency < 1 {
		return Config{}, invalid("max_concurrency", "必须大于等于 1")
	}
	durationFields := []struct {
		name        string
		raw         *string
		target      *time.Duration
		missingUnit string
	}{
		{"scan_interval", raw.ScanInterval, &cfg.ScanInterval, "m"},
		{"activation_request_timeout", raw.ActivationRequestTimeout, &cfg.ActivationRequestTimeout, "s"},
	}
	for _, field := range durationFields {
		if field.raw == nil {
			continue
		}
		if strings.TrimSpace(*field.raw) == "" {
			continue
		}
		parsed, err := parseDuration(field.name, *field.raw, field.missingUnit)
		if err != nil {
			return Config{}, err
		}
		*field.target = parsed
	}
	models, err := raw.ActivationModels.parse(cfg.ActivationModels)
	if err != nil {
		return Config{}, err
	}
	cfg.ActivationModels = models
	return cfg, nil
}

func (raw *rawActivationModels) parse(base ActivationModels) (ActivationModels, error) {
	models := base
	if raw != nil && raw.Codex != nil && raw.Codex.Models != nil {
		models.Codex.Models = strings.TrimSpace(*raw.Codex.Models)
	}
	if raw != nil && raw.Antigravity != nil {
		var err error
		models.Antigravity, err = raw.Antigravity.parse(models.Antigravity)
		if err != nil {
			return ActivationModels{}, err
		}
	}
	return models, nil
}

func parseActivationTransport(raw string) (ActivationTransport, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ActivationTransportDirectHTTP, nil
	}
	switch ActivationTransport(value) {
	case ActivationTransportDirectHTTP, ActivationTransportSchedulerBoost:
		return ActivationTransport(value), nil
	default:
		return "", invalid("activation_transport", "必须是 direct_http 或 scheduler_boost")
	}
}

func parseDuration(field string, raw string, missingUnit string) (time.Duration, error) {
	text := strings.TrimSpace(raw)
	if missingUnit == "" {
		missingUnit = "m"
	}
	if _, err := time.ParseDuration(text); err != nil && strings.Contains(err.Error(), "missing unit") {
		text += missingUnit
	}
	parsed, err := time.ParseDuration(text)
	if err != nil {
		unitHint := "分钟"
		if missingUnit == "s" {
			unitHint = "秒"
		}
		return 0, invalid(field, fmt.Sprintf("必须是可解析的时长字符串，纯数字按%s解析", unitHint))
	}
	if parsed <= 0 {
		return 0, invalid(field, "必须大于 0")
	}
	return parsed, nil
}

func invalid(field string, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidConfig, field, reason)
}
