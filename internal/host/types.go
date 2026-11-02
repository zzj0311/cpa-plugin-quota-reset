package host

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrModelExecuteStatus 表示 host.model.execute 返回了非 2xx HTTP 状态。
var ErrModelExecuteStatus = errors.New("host model execute status")

// Client 是配额激活逻辑依赖的最小 CPA 宿主适配接口。
type Client interface {
	ModelExecute(ctx context.Context, request ModelExecuteRequest) (ModelExecuteResponse, error)
	// HTTPDo 通过 host.http.do 发送外部 HTTP 请求（纯传输，不含业务规则）。
	HTTPDo(ctx context.Context, request HTTPRequest) (HTTPResponse, error)
	ListAuthFiles(ctx context.Context) ([]AuthFile, error)
	// GetAuthFile 按 auth_index 读取物理凭证完整 JSON（host.auth.get）。
	GetAuthFile(ctx context.Context, authIndex string) (AuthFile, error)
	GetRuntimeAuthFile(ctx context.Context, authIndex string) (AuthFile, error)
	SaveAuthFile(ctx context.Context, name string, data []byte) error
}

// Header 是 host.http.do 请求/响应头映射。
type Header map[string][]string

// HTTPRequest 是 host.http.do 的最小请求形状（Method/URL/Headers/Body）。
type HTTPRequest struct {
	Method  string `json:"Method"`
	URL     string `json:"URL"`
	Headers Header `json:"Headers,omitempty"`
	Body    []byte `json:"Body,omitempty"`
}

// HTTPResponse 是 host.http.do 的最小响应形状。
type HTTPResponse struct {
	StatusCode int    `json:"StatusCode"`
	Headers    Header `json:"Headers,omitempty"`
	Body       []byte `json:"Body,omitempty"`
}

// UnmarshalJSON 兼容官方 StatusCode/Headers/Body(base64) 与历史 status_code/body 形态。
func (r *HTTPResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		StatusCode      *int    `json:"StatusCode"`
		StatusCodeSnake *int    `json:"status_code"`
		Headers         Header  `json:"Headers"`
		HeadersLower    Header  `json:"headers"`
		Body            *string `json:"Body"`
		BodyLower       *string `json:"body"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.StatusCode != nil {
		r.StatusCode = *raw.StatusCode
	} else if raw.StatusCodeSnake != nil {
		r.StatusCode = *raw.StatusCodeSnake
	}
	if raw.Headers != nil {
		r.Headers = raw.Headers
	} else {
		r.Headers = raw.HeadersLower
	}
	body, err := decodeModelExecuteBody(raw.Body, raw.BodyLower)
	if err != nil {
		return err
	}
	r.Body = body
	return nil
}

// ModelExecuteRequest 表示可安全脱敏的 host.model.execute 输入形状。
type ModelExecuteRequest struct {
	EntryProtocol string              `json:"entry_protocol,omitempty"`
	ExitProtocol  string              `json:"exit_protocol,omitempty"`
	Model         string              `json:"model"`
	Stream        bool                `json:"stream"`
	Body          []byte              `json:"body"`
	Headers       map[string][]string `json:"headers"`
	Query         map[string][]string `json:"query,omitempty"`
	Alt           string              `json:"alt,omitempty"`
}

// ModelExecuteResponse 表示 host.model.execute 返回的 HTTP 形状结果。
type ModelExecuteResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

// UnmarshalJSON 兼容官方 StatusCode/Headers/Body(base64) 和历史 status_code/body 形态。
func (r *ModelExecuteResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		StatusCode      *int                `json:"StatusCode"`
		StatusCodeSnake *int                `json:"status_code"`
		Headers         map[string][]string `json:"Headers"`
		HeadersLower    map[string][]string `json:"headers"`
		Body            *string             `json:"Body"`
		BodyLower       *string             `json:"body"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.StatusCode != nil {
		r.StatusCode = *raw.StatusCode
	} else if raw.StatusCodeSnake != nil {
		r.StatusCode = *raw.StatusCodeSnake
	}
	if raw.Headers != nil {
		r.Headers = raw.Headers
	} else {
		r.Headers = raw.HeadersLower
	}
	body, err := decodeModelExecuteBody(raw.Body, raw.BodyLower)
	if err != nil {
		return err
	}
	r.Body = body
	return nil
}

func decodeModelExecuteBody(official *string, legacy *string) ([]byte, error) {
	if official != nil {
		if *official == "" {
			return nil, nil
		}
		decoded, err := base64.StdEncoding.DecodeString(*official)
		if err != nil {
			return nil, fmt.Errorf("decode Body base64: %w", err)
		}
		return decoded, nil
	}
	if legacy == nil || *legacy == "" {
		return nil, nil
	}
	if looksLikeBase64(*legacy) {
		if decoded, err := base64.StdEncoding.DecodeString(*legacy); err == nil {
			return decoded, nil
		}
	}
	return []byte(*legacy), nil
}

func looksLikeBase64(value string) bool {
	trimmed := strings.TrimSpace(value)
	return len(trimmed)%4 == 0 && !strings.ContainsAny(trimmed, "{}<>\n\r")
}

// AuthFile 表示宿主凭证存储中的一个凭证元数据或原始凭证文档。
type AuthFile struct {
	ID           string         `json:"id,omitempty"`
	Name         string         `json:"name"`
	AuthIndex    string         `json:"auth_index"`
	Account      string         `json:"account,omitempty"`
	Email        string         `json:"email,omitempty"`
	Provider     string         `json:"provider,omitempty"`
	Type         string         `json:"type,omitempty"`
	Status       string         `json:"status,omitempty"`
	Disabled     bool           `json:"disabled"`
	Unavailable  bool           `json:"unavailable,omitempty"`
	// Cooldowns is CPA local routing cooldown state from auth-files / get_runtime.
	Cooldowns    []any          `json:"cooldowns,omitempty"`
	// Priority 是宿主路由优先级；plugin scheduler 候选仅含同模型最高 priority 层。
	Priority     int            `json:"priority,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Attributes   map[string]any `json:"attributes,omitempty"`
	RecentModels []string       `json:"recent_models,omitempty"`
	Models       []string       `json:"models,omitempty"`
	Data         []byte         `json:"data,omitempty"`
}

// HTTPStatusError 暴露 HTTP 状态码，但不包含请求体或响应体。
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("host model execute returned HTTP status %d", e.StatusCode)
}

func (e *HTTPStatusError) Is(target error) bool {
	return target == ErrModelExecuteStatus
}

// ResponseOrStatusError 返回 2xx 响应，并将非 2xx 状态包装为状态错误。
func ResponseOrStatusError(response ModelExecuteResponse) (ModelExecuteResponse, error) {
	if IsHTTPSuccess(response.StatusCode) {
		return response, nil
	}
	return ModelExecuteResponse{}, fmt.Errorf("model execute: %w", &HTTPStatusError{StatusCode: response.StatusCode})
}

// IsHTTPSuccess 判断 statusCode 是否处于 HTTP 2xx 成功范围。
func IsHTTPSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode <= 299
}
