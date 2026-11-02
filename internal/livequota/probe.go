package livequota

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"quota-activation/internal/activator"
	"quota-activation/internal/host"
)

// DefaultCodexUsageURL is the ChatGPT usage endpoint used by the dashboard Quota page.
const DefaultCodexUsageURL = "https://chatgpt.com/backend-api/wham/usage"

// ProbeCodexLive loads physical auth material and GETs wham/usage via host.http.do.
func ProbeCodexLive(ctx context.Context, client host.Client, authLookupKeys []string, usageURL string, threshold float64) (CodexProbeResult, error) {
	if client == nil {
		return CodexProbeResult{}, fmt.Errorf("live quota: host client required")
	}
	url := strings.TrimSpace(usageURL)
	if url == "" {
		url = DefaultCodexUsageURL
	}
	physical, err := loadPhysical(ctx, client, authLookupKeys)
	if err != nil {
		return CodexProbeResult{}, err
	}
	material, err := activator.ParseAuthMaterial(physical.Data)
	if err != nil {
		return CodexProbeResult{}, err
	}
	if strings.TrimSpace(material.AccessToken) == "" {
		return CodexProbeResult{}, fmt.Errorf("live quota: access token missing")
	}
	headers := host.Header{
		"Authorization": {"Bearer " + material.AccessToken},
		"User-Agent":    {"codex-cli/1.0.0"},
	}
	if accountID := strings.TrimSpace(material.AccountID); accountID != "" {
		headers["Chatgpt-Account-Id"] = []string{accountID}
	}
	resp, err := client.HTTPDo(ctx, host.HTTPRequest{
		Method:  http.MethodGet,
		URL:     url,
		Headers: headers,
	})
	if err != nil {
		return CodexProbeResult{}, fmt.Errorf("live quota probe: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(string(resp.Body))
		if len(msg) > 160 {
			msg = msg[:160]
		}
		return CodexProbeResult{}, fmt.Errorf("live quota HTTP %d: %s", resp.StatusCode, msg)
	}
	return EvaluateCodexUsage(resp.Body, threshold)
}

func loadPhysical(ctx context.Context, client host.Client, keys []string) (host.AuthFile, error) {
	seen := map[string]struct{}{}
	tryGet := func(key string) (host.AuthFile, bool) {
		key = strings.TrimSpace(key)
		if key == "" {
			return host.AuthFile{}, false
		}
		if _, ok := seen[key]; ok {
			return host.AuthFile{}, false
		}
		seen[key] = struct{}{}
		file, err := client.GetAuthFile(ctx, key)
		if err != nil || len(file.Data) == 0 {
			return host.AuthFile{}, false
		}
		return file, true
	}
	for _, key := range keys {
		if file, ok := tryGet(key); ok {
			return file, nil
		}
	}
	// Auto-scan authID is the credential filename (scheduler id), but host.auth.get
	// expects auth_index. Resolve filename/id → auth_index via list, then retry get.
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return host.AuthFile{}, fmt.Errorf("live quota: auth file not found")
	}
	want := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			want[key] = struct{}{}
		}
	}
	for _, file := range files {
		candidates := []string{file.AuthIndex, file.Name, file.ID}
		matched := false
		for _, c := range candidates {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if _, ok := want[c]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if idx := strings.TrimSpace(file.AuthIndex); idx != "" {
			if got, ok := tryGet(idx); ok {
				return got, nil
			}
		}
		if len(file.Data) > 0 {
			return file, nil
		}
	}
	return host.AuthFile{}, fmt.Errorf("live quota: auth file not found")
}

// ResolveAuthIndex returns the CPA auth_index hash for reset-quota.
func ResolveAuthIndex(ctx context.Context, client host.Client, keys []string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("auth_index: host client required")
	}
	seen := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if runtime, err := client.GetRuntimeAuthFile(ctx, key); err == nil {
			if idx := strings.TrimSpace(runtime.AuthIndex); idx != "" {
				return idx, nil
			}
		}
		if physical, err := client.GetAuthFile(ctx, key); err == nil {
			if idx := strings.TrimSpace(physical.AuthIndex); idx != "" {
				return idx, nil
			}
		}
	}
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return "", fmt.Errorf("auth_index list: %w", err)
	}
	want := seen
	for _, file := range files {
		candidates := []string{file.AuthIndex, file.Name, file.ID}
		for _, c := range candidates {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if _, ok := want[c]; ok {
				if idx := strings.TrimSpace(file.AuthIndex); idx != "" {
					return idx, nil
				}
			}
		}
		// Also match when lookup key equals name but AuthIndex differs.
		for key := range want {
			if strings.TrimSpace(file.Name) == key || strings.TrimSpace(file.ID) == key {
				if idx := strings.TrimSpace(file.AuthIndex); idx != "" {
					return idx, nil
				}
			}
		}
	}
	return "", fmt.Errorf("auth_index not found")
}

// HasLocalBlock reports whether runtime auth is locally cooled down / unavailable.
func HasLocalBlock(file host.AuthFile) bool {
	if file.Unavailable || len(file.Cooldowns) > 0 {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(file.Status))
	return status == "error" || status == "unavailable"
}
