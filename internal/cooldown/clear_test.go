package cooldown

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"quota-activation/internal/host"
)

type fakeHTTP struct {
	last host.HTTPRequest
	resp host.HTTPResponse
	err  error
}

func (f *fakeHTTP) ModelExecute(context.Context, host.ModelExecuteRequest) (host.ModelExecuteResponse, error) {
	return host.ModelExecuteResponse{}, nil
}
func (f *fakeHTTP) HTTPDo(_ context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	f.last = req
	return f.resp, f.err
}
func (f *fakeHTTP) ListAuthFiles(context.Context) ([]host.AuthFile, error) { return nil, nil }
func (f *fakeHTTP) GetAuthFile(context.Context, string) (host.AuthFile, error) {
	return host.AuthFile{}, nil
}
func (f *fakeHTTP) GetRuntimeAuthFile(context.Context, string) (host.AuthFile, error) {
	return host.AuthFile{}, nil
}
func (f *fakeHTTP) SaveAuthFile(context.Context, string, []byte) error { return nil }

func TestClearLocal_PostsAuthIndex(t *testing.T) {
	f := &fakeHTTP{resp: host.HTTPResponse{StatusCode: 200, Body: []byte(`{"status":"ok"}`)}}
	err := ClearLocal(context.Background(), f, Options{
		URL:         "http://127.0.0.1:8317/v0/management/reset-quota",
		AuthIndex:   "abc123",
		BearerToken: "secret",
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if f.last.Method != http.MethodPost {
		t.Fatalf("method %s", f.last.Method)
	}
	if f.last.URL != "http://127.0.0.1:8317/v0/management/reset-quota" {
		t.Fatalf("url %s", f.last.URL)
	}
	auth := ""
	if vals := f.last.Headers["Authorization"]; len(vals) > 0 {
		auth = vals[0]
	}
	if auth != "Bearer secret" {
		t.Fatalf("auth %q", auth)
	}
	var body map[string]string
	if err := json.Unmarshal(f.last.Body, &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body["auth_index"] != "abc123" {
		t.Fatalf("body %+v", body)
	}
}

func TestClearLocal_Non2xx(t *testing.T) {
	f := &fakeHTTP{resp: host.HTTPResponse{StatusCode: 400, Body: []byte(`{"error":"auth_index is required"}`)}}
	err := ClearLocal(context.Background(), f, Options{
		URL:         "http://127.0.0.1:8317/v0/management/reset-quota",
		AuthIndex:   "x",
		BearerToken: "k",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClearLocal_ViaHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]string
		_ = json.Unmarshal(raw, &body)
		if body["auth_index"] != "idx" {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	client := &loopbackHTTP{base: srv.Client()}
	err := ClearLocal(context.Background(), client, Options{
		URL:         srv.URL,
		AuthIndex:   "idx",
		BearerToken: "tok",
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
}

type loopbackHTTP struct{ base *http.Client }

func (l *loopbackHTTP) ModelExecute(context.Context, host.ModelExecuteRequest) (host.ModelExecuteResponse, error) {
	return host.ModelExecuteResponse{}, nil
}
func (l *loopbackHTTP) HTTPDo(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return host.HTTPResponse{}, err
	}
	for k, vals := range req.Headers {
		for _, v := range vals {
			httpReq.Header.Add(k, v)
		}
	}
	res, err := l.base.Do(httpReq)
	if err != nil {
		return host.HTTPResponse{}, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return host.HTTPResponse{StatusCode: res.StatusCode, Body: raw}, nil
}
func (l *loopbackHTTP) ListAuthFiles(context.Context) ([]host.AuthFile, error) { return nil, nil }
func (l *loopbackHTTP) GetAuthFile(context.Context, string) (host.AuthFile, error) {
	return host.AuthFile{}, nil
}
func (l *loopbackHTTP) GetRuntimeAuthFile(context.Context, string) (host.AuthFile, error) {
	return host.AuthFile{}, nil
}
func (l *loopbackHTTP) SaveAuthFile(context.Context, string, []byte) error { return nil }
