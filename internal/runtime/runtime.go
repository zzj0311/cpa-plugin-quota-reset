package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quota-activation/internal/activator"
	"quota-activation/internal/config"
	"quota-activation/internal/host"
	"quota-activation/internal/management"
	"quota-activation/internal/scheduler"
	"quota-activation/internal/session"
	"quota-activation/internal/state"
)

// Runtime 持有 CPA 插件生命周期、配置和调度/管理依赖。
type Runtime struct {
	mu   sync.Mutex
	host host.Client
	now  func() time.Time
	// sleep 可注入：sleep(ctx, d) 在 d 内阻塞，ctx 取消返回 false。
	sleep            func(context.Context, time.Duration) bool
	config           config.Config
	sessions         *session.Manager
	store            *state.Store
	activator        *activator.Activator
	management       *management.Handler
	picker           scheduler.Picker
	autoCancel       context.CancelFunc
	autoScan         func(context.Context)
	autoScanInterval time.Duration
	// lastAutoScanAt 最近一次实际执行 auto scan 的时间（mu 保护）；门闩强制遵守 ScanInterval。
	lastAutoScanAt time.Time
	// autoScanRunning 防重入：上一轮未完成时拒绝新一轮 claim。
	autoScanRunning bool
	// startupDelay nil 表示默认 autoScanStartupDelay；非 nil 为测试注入（含 0）。
	startupDelay *time.Duration
	// schedulerMissUntil 记录可冷却失败（调度未选中 / 宿主执行器不可用）的 per-auth 截止时间。
	schedulerMissUntil map[string]time.Time
	// activateCooldownReasonByAuth 冷却 skip 原因（与 schedulerMissUntil 同步）。
	activateCooldownReasonByAuth map[string]string
	runHistory                   []RunHistoryEntry
	lastUsableSuccess            map[usableAuthKey]state.Record
	shutdown                     bool
}

// New 创建未注册 runtime；首次 register/reconfigure 成功后才启用依赖。
func New(options Options) *Runtime {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	sleep := options.Sleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	runtime := &Runtime{host: options.Host, now: now, sleep: sleep, config: config.Default(), startupDelay: options.StartupDelay}
	runtime.autoScan = runtime.runAutoScanLoop
	return runtime
}

// sleepWithContext 在 d 内阻塞；ctx 取消返回 false。
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Handle 根据 CPA 方法名处理 JSON 请求并返回 JSON 信封字节。
func (r *Runtime) Handle(ctx context.Context, method string, request []byte) []byte {
	switch method {
	case "plugin.register":
		parsed, err := decodeRegisterRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Register(ctx, parsed)
		return envelopeResult(result, err)
	case "plugin.reconfigure":
		parsed, err := decodeReconfigureRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Reconfigure(ctx, parsed)
		return envelopeResult(result, err)
	case "plugin.shutdown":
		return envelopeStatus(r.Shutdown(ctx))
	case "management.register":
		return r.registerManagement()
	case "management.handle":
		return r.handleManagement(ctx, request)
	case "scheduler.pick":
		return r.pickSchedule(request)
	default:
		return failure(fmt.Errorf("%w: method %q", ErrInvalidRequest, method))
	}
}

// Register 解析首次配置并返回 management_api + scheduler 能力声明。
func (r *Runtime) Register(ctx context.Context, request RegisterRequest) (RegisterResult, error) {
	cfg, err := config.Parse([]byte(request.ConfigYAML))
	if err != nil {
		return RegisterResult{}, fmt.Errorf("load register config: %w", err)
	}
	if err := r.replaceConfig(ctx, cfg); err != nil {
		return RegisterResult{}, err
	}
	return registrationResult(), nil
}

// Reconfigure 验证并替换运行时配置。
func (r *Runtime) Reconfigure(ctx context.Context, request ReconfigureRequest) (RegisterResult, error) {
	cfg, err := config.Parse([]byte(request.ConfigYAML))
	if err != nil {
		return RegisterResult{}, fmt.Errorf("load reconfigure config: %w", err)
	}
	if err := r.replaceConfig(ctx, cfg); err != nil {
		return RegisterResult{}, err
	}
	return registrationResult(), nil
}

// Shutdown 将 runtime 标记为关闭，并等待在途 auto scan 结束（避免 TempDir/状态写盘竞态）。
func (r *Runtime) Shutdown(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("runtime shutdown context: %w", err)
	}
	r.mu.Lock()
	r.stopAutoScannerLocked()
	r.shutdown = true
	r.mu.Unlock()
	// 等在途 scan/Activate/SaveAtomic 结束，防止测试 TempDir 清理撞上 .tmp 写盘。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		running := r.autoScanRunning
		r.mu.Unlock()
		if !running {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("runtime shutdown context: %w", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func (r *Runtime) startAutoScannerLocked(interval time.Duration) {
	if interval <= 0 {
		interval = config.Default().ScanInterval
	}
	// 已在跑且 interval 相同 → no-op（宿主频繁 reconfigure 的主路径）。
	if r.autoCancel != nil && r.autoScanInterval == interval {
		return
	}
	r.stopAutoScannerLocked()
	ctx, cancel := context.WithCancel(context.Background())
	r.autoCancel = cancel
	r.autoScanInterval = interval
	go r.autoScan(ctx)
}

func (r *Runtime) stopAutoScannerLocked() {
	if r.autoCancel == nil {
		return
	}
	r.autoCancel()
	r.autoCancel = nil
	r.autoScanInterval = 0
}

func registrationResult() RegisterResult {
	return RegisterResult{
		SchemaVersion: 1,
		Metadata: Metadata{
			Name:             "quota-reset",
			Version:          "0.0.12",
			Author:           "cpaDash",
			GitHubRepository: "https://github.com/zzj0311/cpa-plugin-quota-reset",
			Description:      "Quota reset activation with live Codex usage gate and local cooldown clear (cpaDash fork).",
			ConfigFields:     configFields(),
		},
		Capabilities: map[string]bool{"management_api": true, "scheduler": true},
	}
}

func configFields() []ConfigField {
	defaults := config.Default()
	return []ConfigField{
		{Name: "activation_models.codex.models", Type: "string", Description: localizedDescription("Codex 自动唤醒使用的模型名称。", "Model name used for Codex automatic activation."), DefaultValue: defaults.ActivationModels.Codex.Models},
		{Name: "activation_models.antigravity.models_group", Type: "string", Description: localizedDescription("Antigravity 自动唤醒模型组（gemini、claude_gpt、gemini_4_argon）。支持 1~3 个模型组并用 | 分隔，与 models 按位置配对；每个组只能一个模型；旧单值仍兼容；不支持数组/逗号。", "Model group for Antigravity automatic activation (gemini, claude_gpt, gemini_4_argon). Supports 1-3 groups separated by | and paired with models by position; one model per group; legacy single value compatible; arrays/commas not supported."), DefaultValue: defaults.ActivationModels.Antigravity.ModelsGroup},
		{Name: "activation_models.antigravity.models", Type: "string", Description: localizedDescription("Antigravity 自动唤醒使用的模型名称。多组时用 | 分隔并与 models_group 按位置配对；每个组只能一个模型；旧单值仍兼容；不支持数组/逗号。", "Model name used for Antigravity automatic activation. For multiple groups, separate with | and pair with models_group by position; one model per group; legacy single value compatible; arrays/commas not supported."), DefaultValue: defaults.ActivationModels.Antigravity.Models},
		{Name: "auto_activate", Type: "boolean", Description: localizedDescription("启用调度器自动配额唤醒；默认 false，因此手动唤醒仍是默认方式。未启用时执行记录不会出现 auto scan 摘要。", "Enable scheduler-driven automatic quota activation; default is false, so manual activation remains the default. When disabled, no auto scan summary appears in run history."), DefaultValue: defaults.AutoActivate},
		{Name: "enable_before_activation", Type: "boolean", Description: localizedDescription("默认 true：达到唤醒条件后自动启用已禁用凭证并保持启用；显式 false 时候选阶段丢弃 disabled。", "Default true: automatically enable disabled credentials that meet activation conditions and keep them enabled; explicit false drops them at candidate stage."), DefaultValue: defaults.EnableBeforeActivation},
		{Name: "scan_interval", Type: "string", Description: localizedDescription("自动扫描间隔（单位：分钟）。填写纯数字即可，无需带单位；默认 30。", "Automatic scan interval in minutes. Enter a plain number without a unit; default is 30."), DefaultValue: formatDurationMinutes(defaults.ScanInterval)},
		{Name: "activation_request_timeout", Type: "string", Description: localizedDescription("唤醒模型请求超时（单位：秒）。填写纯数字即可，无需带单位；默认 60。", "Activation model request timeout in seconds. Enter a plain number without a unit; default is 60."), DefaultValue: formatDurationSeconds(defaults.ActivationRequestTimeout)},
		{Name: "max_concurrency", Type: "integer", Description: localizedDescription("自动扫描与激活的并发上限（worker 池大小）；默认 1。", "Concurrency limit for automatic scan activation (worker pool size); default 1."), DefaultValue: defaults.MaxConcurrency},
		{Name: "activation_prompt", Type: "string", Description: localizedDescription("通过 host.model.execute 发送的配额唤醒提示词。", "Prompt sent through host.model.execute for quota activation."), DefaultValue: defaults.ActivationPrompt},
		{Name: "activation_transport", Type: "string", Description: localizedDescription("唤醒传输方式：direct_http（默认，经 host.http.do）或 scheduler_boost。", "Activation transport: direct_http (default, via host.http.do) or scheduler_boost."), EnumValues: []string{string(config.ActivationTransportDirectHTTP), string(config.ActivationTransportSchedulerBoost)}, DefaultValue: string(defaults.ActivationTransport)},
		{Name: "scheduler_boost_fallback", Type: "boolean", Description: localizedDescription("direct_http 遇传输/宿主类失败时是否回退 scheduler_boost；默认 true。业务失败不回退。", "When direct_http hits transport/host failures, fall back to scheduler_boost; default true. Business failures never fall back."), DefaultValue: defaults.SchedulerBoostFallback},
		{Name: "require_live_quota_for_auto", Type: "boolean", Description: localizedDescription("自动路径必须先通过 live Codex usage 探针（used_percent < 阈值）才唤醒/清冷却；默认 true。", "Auto path requires a live Codex usage probe (used_percent under threshold) before wake/cooldown clear; default true."), DefaultValue: defaults.RequireLiveQuotaForAuto},
		{Name: "clear_local_cooldown", Type: "boolean", Description: localizedDescription("确认额度恢复后调用 reset-quota 清除 CPA 本地冷却；默认 true。", "After confirmed recovery, call reset-quota to clear CPA local cooldowns; default true."), DefaultValue: defaults.ClearLocalCooldown},
		{Name: "management_loopback_url", Type: "string", Description: localizedDescription("CPA 进程内 reset-quota 地址。", "In-process CPA reset-quota URL."), DefaultValue: defaults.ManagementLoopbackURL},
		{Name: "management_key", Type: "string", Description: localizedDescription("自动路径调用 reset-quota 使用的管理密钥；手动路径复用请求 Authorization。", "Management key for auto reset-quota; manual path reuses request Authorization."), DefaultValue: ""},
		{Name: "codex_usage_url", Type: "string", Description: localizedDescription("Codex live 用量探针 URL。", "Codex live usage probe URL."), DefaultValue: defaults.CodexUsageURL},
		{Name: "codex_used_percent_threshold", Type: "number", Description: localizedDescription("used_percent 低于该阈值才算恢复；默认 95。", "used_percent must be below this threshold to count as recovered; default 95."), DefaultValue: defaults.CodexUsedPercentThreshold},
	}
}
func localizedDescription(chinese string, english string) string {
	return chinese + "\n" + english
}

func formatDurationMinutes(value time.Duration) string {
	if value <= 0 {
		return "30"
	}
	minutes := int(value / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%d", minutes)
}

func formatDurationSeconds(value time.Duration) string {
	if value <= 0 {
		return "60"
	}
	seconds := int(value / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("%d", seconds)
}
