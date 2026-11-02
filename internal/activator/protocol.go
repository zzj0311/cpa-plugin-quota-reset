package activator

import "quota-activation/internal/host"

// ProtocolRequest 是直连上游的最小 HTTP 请求形状（供 host.http.do 使用）。
// 与具体 provider 解耦的公共载体；Codex/Antigravity 各自独立构建。
type ProtocolRequest struct {
	Method  string
	URL     string
	Headers map[string][]string
	Body    []byte
}

// ToHTTPRequest 转为 host.HTTPRequest（纯传输映射，不含业务规则）。
func (r ProtocolRequest) ToHTTPRequest() host.HTTPRequest {
	headers := host.Header(nil)
	if len(r.Headers) > 0 {
		headers = make(host.Header, len(r.Headers))
		for key, values := range r.Headers {
			copied := make([]string, len(values))
			copy(copied, values)
			headers[key] = copied
		}
	}
	body := append([]byte(nil), r.Body...)
	return host.HTTPRequest{
		Method:  r.Method,
		URL:     r.URL,
		Headers: headers,
		Body:    body,
	}
}
