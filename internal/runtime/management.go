package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"quota-activation/internal/management"
)

// ManagementRequest 是 CPA management.handle 调用传入的 HTTP 请求信封。
type ManagementRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Query   url.Values
	Body    []byte
}

// ManagementResponse 是 management.handle 返回给 CPA 宿主的 HTTP 结果信封。
type ManagementResponse struct {
	StatusCode  int                 `json:"StatusCode"`
	ContentType string              `json:"content_type"`
	Headers     map[string][]string `json:"Headers"`
	Body        string              `json:"Body"`
}

type managementRoute struct {
	Method string `json:"Method"`
	Path   string `json:"Path"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

type managementRegistration struct {
	Routes    []managementRoute    `json:"routes"`
	Resources []managementResource `json:"resources"`
}

func (r *Runtime) registerManagement() []byte {
	registered := management.Register()
	routes := make([]managementRoute, 0, len(registered.Routes))
	for _, route := range registered.Routes {
		routes = append(routes, managementRoute{Method: route.Method, Path: route.Path})
	}
	resources := make([]managementResource, 0, len(registered.Resources))
	for _, resource := range registered.Resources {
		resources = append(resources, managementResource{Path: resource.Path, Menu: resource.Menu, Description: resource.Description})
	}
	return envelopeResult(managementRegistration{Routes: routes, Resources: resources}, nil)
}

func (r *Runtime) handleManagement(ctx context.Context, raw []byte) []byte {
	handler, err := r.managementHandler()
	if err != nil {
		return failure(err)
	}
	request, err := decodeManagementRequest(raw)
	if err != nil {
		return failure(err)
	}
	httpRequest, err := request.toHTTPRequest(ctx)
	if err != nil {
		return failure(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httpRequest)
	return envelopeResult(newManagementResponse(recorder), nil)
}

func (r *Runtime) managementHandler() (*management.Handler, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shutdown {
		return nil, ErrShutdown
	}
	if r.management == nil {
		return nil, fmt.Errorf("%w: management is not configured", ErrInvalidRequest)
	}
	return r.management, nil
}

func decodeManagementRequest(raw []byte) (ManagementRequest, error) {
	if len(raw) == 0 {
		return ManagementRequest{}, fmt.Errorf("%w: management request is required", ErrInvalidRequest)
	}
	var wire managementRequestWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ManagementRequest{}, fmt.Errorf("%w: decode management request: %v", ErrInvalidRequest, err)
	}
	return wire.toManagementRequest()
}

type managementRequestWire struct {
	Method      string      `json:"Method"`
	MethodLower string      `json:"method"`
	Path        string      `json:"Path"`
	PathLower   string      `json:"path"`
	Headers     http.Header `json:"Headers"`
	Query       url.Values  `json:"Query"`
	QueryLower  string      `json:"query"`
	Body        string      `json:"Body"`
	BodyLower   string      `json:"body"`
}

func (w managementRequestWire) toManagementRequest() (ManagementRequest, error) {
	body, err := decodeManagementBody(w.Body, w.BodyLower)
	if err != nil {
		return ManagementRequest{}, err
	}
	query := w.Query
	if query == nil && strings.TrimSpace(w.QueryLower) != "" {
		parsed, err := url.ParseQuery(strings.TrimPrefix(w.QueryLower, "?"))
		if err != nil {
			return ManagementRequest{}, fmt.Errorf("%w: decode management query: %v", ErrInvalidRequest, err)
		}
		query = parsed
	}
	request := ManagementRequest{Method: firstNonEmpty(w.Method, w.MethodLower), Path: firstNonEmpty(w.Path, w.PathLower), Headers: w.Headers, Query: query, Body: body}
	if request.Method == "" || request.Path == "" {
		return ManagementRequest{}, fmt.Errorf("%w: management method and path are required", ErrInvalidRequest)
	}
	return request, nil
}

func decodeManagementBody(official string, legacy string) ([]byte, error) {
	if official != "" {
		decoded, err := base64.StdEncoding.DecodeString(official)
		if err != nil {
			return nil, fmt.Errorf("%w: decode management body: %v", ErrInvalidRequest, err)
		}
		return decoded, nil
	}
	return []byte(legacy), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (r ManagementRequest) toHTTPRequest(ctx context.Context) (*http.Request, error) {
	path := normalizeManagementPath(r.Path)
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("%w: management path must start with /", ErrInvalidRequest)
	}
	if r.Query != nil && r.Query.Encode() != "" {
		path += "?" + r.Query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, r.Method, path, bytes.NewBuffer(r.Body))
	if err != nil {
		return nil, fmt.Errorf("%w: build management request: %v", ErrInvalidRequest, err)
	}
	request.Header = r.Headers
	return request, nil
}

func normalizeManagementPath(path string) string {
	const resourcePrefix = "/v0/resource/plugins/quota-reset"
	if path == resourcePrefix {
		return "/"
	}
	if strings.HasPrefix(path, resourcePrefix+"/") {
		return strings.TrimPrefix(path, resourcePrefix)
	}
	if path == "/plugins/quota-reset" {
		return "/v0/management/quota-reset"
	}
	if strings.HasPrefix(path, "/plugins/quota-reset/") {
		return "/v0/management/quota-reset" + strings.TrimPrefix(path, "/plugins/quota-reset")
	}
	return path
}

func newManagementResponse(recorder *httptest.ResponseRecorder) ManagementResponse {
	result := recorder.Result()
	return ManagementResponse{StatusCode: result.StatusCode, ContentType: result.Header.Get("Content-Type"), Headers: result.Header, Body: base64.StdEncoding.EncodeToString(recorder.Body.Bytes())}
}
