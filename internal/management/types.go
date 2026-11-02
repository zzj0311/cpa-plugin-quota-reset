package management

import (
	"net/http"
	"sync"
	"time"

	"quota-activation/internal/activator"
	"quota-activation/internal/config"
	"quota-activation/internal/host"
	"quota-activation/internal/state"
)

const managementPrefix = "/v0/management/quota-reset"

const resourceStatusPath = "/status"

// Options 定义管理 API 处理器依赖。
type Options struct {
	Activator *activator.Activator
	Host      host.Client
	// Store 可选：手动激活 Evaluate 时注入 previous，减少无意义 Activate 调用。
	// 主幂等仍由 Activator 硬闸保证；Store 为 nil 时仅依赖硬闸。
	Store  *state.Store
	Config config.Config
	Now    func() time.Time
	// OnActivation 在每次手动激活完成后回调（成功或失败均触发），用于写入 run_history。
	OnActivation func(result activator.Result, err error)
	// RunHistory 返回 diagnostics 内嵌的 run_history 数组（任意 JSON 可序列化切片）。
	RunHistory func() any
}

// Route 描述管理 API 暴露的 HTTP 路由。
type Route struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// Resource 描述 CPA 管理端可直接打开的资源入口。
type Resource struct {
	Path        string `json:"path"`
	Menu        string `json:"menu"`
	Description string `json:"description"`
}

// Registration 是宿主注册管理能力时使用的脱敏路由清单。
type Registration struct {
	Routes    []Route    `json:"routes"`
	Resources []Resource `json:"resources"`
}

// Handler 处理 quota-activation 的管理 API HTTP 请求。
type Handler struct {
	activator    *activator.Activator
	host         host.Client
	store        *state.Store
	config       config.Config
	now          func() time.Time
	onActivation func(result activator.Result, err error)
	runHistory   func() any
	mu           sync.RWMutex
	latest       activationResponse
}

// Register 返回 quota-activation 的管理 API 路由清单和插件资源页。
func Register() Registration {
	return Registration{
		Routes: []Route{
			{Method: http.MethodGet, Path: managementPrefix},
			{Method: http.MethodGet, Path: managementPrefix + "/status"},
			{Method: http.MethodGet, Path: managementPrefix + "/auth-files"},
			{Method: http.MethodPost, Path: managementPrefix + "/activate"},
			{Method: http.MethodGet, Path: managementPrefix + "/diagnostics"},
		},
		Resources: []Resource{
			{Path: resourceStatusPath, Menu: "Quota Activation Local", Description: "查看 quota-reset 状态并手动触发配额唤醒。"},
		},
	}
}

// NewHandler 构造标准库 net/http 管理 API 处理器。
func NewHandler(options Options) *Handler {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{
		activator:    options.Activator,
		host:         options.Host,
		store:        options.Store,
		config:       options.Config,
		now:          now,
		onActivation: options.OnActivation,
		runHistory:   options.RunHistory,
	}
}
