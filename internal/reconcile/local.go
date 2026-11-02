package reconcile

import (
	"context"
	"fmt"
	"strings"

	"quota-activation/internal/cooldown"
	"quota-activation/internal/config"
	"quota-activation/internal/detector"
	"quota-activation/internal/host"
	"quota-activation/internal/livequota"
)

// CodexGate runs the live Codex usage probe when provider is codex.
// Non-codex providers are treated as recovered=true (no live gate in v1).
func CodexGate(ctx context.Context, client host.Client, cfg config.Config, provider detector.Provider, lookupKeys []string) (livequota.CodexProbeResult, error) {
	if provider != detector.ProviderCodex {
		return livequota.CodexProbeResult{Recovered: true}, nil
	}
	threshold := cfg.CodexUsedPercentThreshold
	if threshold <= 0 {
		threshold = livequota.DefaultUsedPercentThreshold
	}
	return livequota.ProbeCodexLive(ctx, client, lookupKeys, cfg.CodexUsageURL, threshold)
}

// ClearIfNeeded calls reset-quota when config allows and conditions match.
func ClearIfNeeded(ctx context.Context, client host.Client, cfg config.Config, lookupKeys []string, bearer string, recovered, localBlock, wakeOK bool) (cleared bool, err error) {
	if !cfg.ClearLocalCooldown {
		return false, nil
	}
	if !livequota.ShouldClearLocalCooldown(recovered, localBlock, wakeOK) {
		return false, nil
	}
	token := strings.TrimSpace(bearer)
	if token == "" {
		token = strings.TrimSpace(cfg.ManagementKey)
	}
	if token == "" {
		return false, fmt.Errorf("缺少管理密钥，无法清除本地冷却")
	}
	authIndex, err := livequota.ResolveAuthIndex(ctx, client, lookupKeys)
	if err != nil {
		return false, err
	}
	url := cfg.ManagementLoopbackURL
	if strings.TrimSpace(url) == "" {
		url = cooldown.DefaultResetQuotaURL
	}
	if err := cooldown.ClearLocal(ctx, client, cooldown.Options{
		URL:         url,
		AuthIndex:   authIndex,
		BearerToken: token,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// RuntimeLocalBlock loads runtime auth and reports local cooldown/unavailable.
func RuntimeLocalBlock(ctx context.Context, client host.Client, lookupKeys []string) bool {
	if client == nil {
		return false
	}
	for _, key := range lookupKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if runtime, err := client.GetRuntimeAuthFile(ctx, key); err == nil {
			if livequota.HasLocalBlock(runtime) {
				return true
			}
		}
	}
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return false
	}
	want := map[string]struct{}{}
	for _, key := range lookupKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			want[key] = struct{}{}
		}
	}
	for _, file := range files {
		name := strings.TrimSpace(file.Name)
		id := strings.TrimSpace(file.ID)
		idx := strings.TrimSpace(file.AuthIndex)
		_, okName := want[name]
		_, okID := want[id]
		_, okIdx := want[idx]
		if okName || okID || okIdx {
			if livequota.HasLocalBlock(file) {
				return true
			}
		}
	}
	return false
}

// BearerFromRequest extracts Authorization Bearer token.
func BearerFromRequest(header string) string {
	header = strings.TrimSpace(header)
	const prefix = "Bearer "
	if len(header) >= len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}
