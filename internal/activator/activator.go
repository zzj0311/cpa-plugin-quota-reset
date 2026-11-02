package activator

import (
	"context"
	"sync"
	"time"

	"quota-activation/internal/config"
	"quota-activation/internal/host"
	"quota-activation/internal/session"
	"quota-activation/internal/state"
)

// Activator 执行一次性 quota 激活请求；并发上限由 config.MaxConcurrency 信号量控制。
type Activator struct {
	host     host.Client
	sessions *session.Manager
	store    *state.Store
	config   config.Config
	now      func() time.Time
	// runSem 容量 = max(1, MaxConcurrency)；TryAcquire 失败 → StatusBusy。
	runSem chan struct{}
	// providerBoostMu 保护 providerLocks 映射本身。
	providerBoostMu sync.Mutex
	// providerLocks 按 provider 串行化 list→boost save→runtime 确认，不跨 ModelExecute。
	providerLocks map[string]*sync.Mutex
	// sleep 可注入：sleep(ctx, d) 在 d 内阻塞，ctx 取消返回 false。
	sleep             func(context.Context, time.Duration) bool
	usableMu          sync.Mutex
	lastUsableSuccess map[usableAuthKey]state.Record
}

// New 构造纯内部激活执行器。
func New(options Options) *Activator {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Activator{
		host:          options.Host,
		sessions:      options.Sessions,
		store:         options.State,
		config:        options.Config,
		now:           now,
		sleep:         options.Sleep,
		runSem:        make(chan struct{}, maxConcurrencySlots(options.Config.MaxConcurrency)),
		providerLocks: make(map[string]*sync.Mutex),
	}
}

func maxConcurrencySlots(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// Activate 执行一次配额唤醒：默认 direct_http（wakeDirect），可选 scheduler_boost（legacy boost+execute）。
// 禁用凭证统一走 enableIfNeeded 后继续唤醒，不受 enable_before_activation 阻断。
// manual/auto 共用本入口；并发槽满立即 StatusBusy/ErrBusy。
func (a *Activator) Activate(ctx context.Context, request Request) (Result, error) {
	result := a.initialResult(request)
	if !a.tryAcquireRunSlot() {
		result.Status = StatusBusy
		return result, ErrBusy
	}
	defer a.releaseRunSlot()

	if err := a.checkDependencies(); err != nil {
		return a.failAndStore(ctx, result, err)
	}
	normalized, err := a.normalizeRequest(request)
	result = a.initialResult(normalized)
	if err != nil {
		return a.failAndStore(ctx, result, err)
	}

	// 硬闸：manual/auto 共用。周期内已有 success 且未证明 remaining 恢复 → skip，禁止 HTTPDo/boost。
	if skip, reason := shouldHardSkipActiveCycle(a.hardSkipStore(), normalized, a.now().UTC()); skip {
		return a.storeResult(ctx, applyHardSkip(result, reason))
	}

	enabled, enableErr := a.enableIfNeeded(ctx, normalized)
	if enableErr != nil {
		return a.failAndStore(ctx, result, enableErr)
	}
	if enabled {
		result.TemporaryEnabled = true
	}

	// 仅显式 scheduler_boost 走 legacy；空值/其它均按默认 direct_http。
	if a.usesSchedulerBoostTransport() {
		return a.activateViaSchedulerBoost(ctx, normalized, result)
	}
	return a.activateViaDirectHTTP(ctx, normalized, result)
}

// usesSchedulerBoostTransport 判定是否走 legacy boost 路径（默认 false → direct_http）。
func (a *Activator) usesSchedulerBoostTransport() bool {
	if a == nil {
		return false
	}
	return a.config.ActivationTransport == config.ActivationTransportSchedulerBoost
}

// activateViaDirectHTTP 默认主路径：wakeDirect，禁止 priority boost / SaveAuthFile（启用禁用凭证除外）。
// 仅当 SchedulerBoostFallback 且 direct 失败属传输/宿主类时，回退一次 legacy boost。
// 401 三次跳过（StatusSkipped）不得改写为 failed，也不走 boost fallback。
func (a *Activator) activateViaDirectHTTP(ctx context.Context, request Request, result Result) (Result, error) {
	result.WakePath = WakePathDirectHTTP
	result, activateErr := a.wakeDirect(ctx, request, result)
	if activateErr == nil && result.Status == StatusSuccess && result.Success {
		result.WakePath = WakePathDirectHTTP
		return a.storeResult(ctx, result)
	}
	// 鉴权失效跳过：保持 StatusSkipped，计入 skipped 而非 failed。
	if activateErr == nil && result.Status == StatusSkipped {
		result.Success = false
		result.WakePath = WakePathDirectHTTP
		return a.storeResult(ctx, result)
	}
	// 业务/严格失败或传输失败：统一归一为 failed，再决定是否 fallback。
	if result.Status != StatusFailed && result.Status != StatusBusy {
		result.Status = StatusFailed
	}
	result.Success = false
	if a.shouldFallbackToSchedulerBoost(result, activateErr) {
		return a.runSchedulerBoostPath(ctx, request, result, WakePathSchedulerBoostFallback)
	}
	if activateErr != nil {
		return a.failAndStore(ctx, result, activateErr)
	}
	return a.storeResult(ctx, result)
}

// activateViaSchedulerBoost legacy 主路径（显式 activation_transport=scheduler_boost）。
func (a *Activator) activateViaSchedulerBoost(ctx context.Context, request Request, result Result) (Result, error) {
	return a.runSchedulerBoostPath(ctx, request, result, WakePathSchedulerBoost)
}

// runSchedulerBoostPath：boost → execute → restore（restore 必跑，Background 防父 ctx 取消）。
// per-provider 锁在 boost 内持有至 restore，避免并发双顶。
func (a *Activator) runSchedulerBoostPath(ctx context.Context, request Request, result Result, path WakePath) (Result, error) {
	result.WakePath = path
	// 从 direct 失败转入时清掉旧失败态，避免 execute 成功后仍被 LastError 误判。
	result.Status = ""
	result.Success = false
	result.LastError = ""

	restorePriority, boostErr := a.boostPriorityForSelection(ctx, request.AuthID)
	if boostErr != nil {
		return a.failAndStore(ctx, result, boostErr)
	}
	defer restorePriority(context.Background())

	result, activateErr := a.execute(ctx, request, result)
	if activateErr != nil {
		result.WakePath = path
		return a.failAndStore(ctx, result, activateErr)
	}
	if result.Status == StatusFailed && result.LastError != "" {
		result.WakePath = path
		return a.storeResult(ctx, result)
	}
	result.Status = StatusSuccess
	result.Success = true
	result.LastError = ""
	result.WakePath = path
	return a.storeResult(ctx, result)
}

func (a *Activator) tryAcquireRunSlot() bool {
	if a == nil || a.runSem == nil {
		return true
	}
	select {
	case a.runSem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (a *Activator) releaseRunSlot() {
	if a == nil || a.runSem == nil {
		return
	}
	select {
	case <-a.runSem:
	default:
	}
}

func (a *Activator) providerBoostLock(provider string) *sync.Mutex {
	key := provider
	if key == "" {
		key = "_"
	}
	a.providerBoostMu.Lock()
	defer a.providerBoostMu.Unlock()
	if a.providerLocks == nil {
		a.providerLocks = make(map[string]*sync.Mutex)
	}
	if mu, ok := a.providerLocks[key]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	a.providerLocks[key] = mu
	return mu
}

func (a *Activator) checkDependencies() error {
	if a == nil || a.host == nil || a.sessions == nil || a.store == nil {
		return ErrMissingDependency
	}
	return nil
}
