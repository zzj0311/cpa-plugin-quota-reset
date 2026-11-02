package livequota

import (
	"encoding/json"
	"fmt"
)

// DefaultUsedPercentThreshold is the max used_percent still treated as recovered.
// Values at or above this threshold are treated as not recovered (avoids false positives).
const DefaultUsedPercentThreshold = 95

// CodexProbeResult is the sanitized outcome of a live wham/usage probe.
type CodexProbeResult struct {
	Recovered          bool
	PrimaryUsedPercent *float64
	SecondaryUsedPercent *float64
	Allowed            *bool
	LimitReached       *bool
}

type codexUsageBody struct {
	RateLimit *codexRateLimit `json:"rate_limit"`
}

type codexRateLimit struct {
	Allowed       *bool       `json:"allowed"`
	LimitReached  *bool       `json:"limit_reached"`
	PrimaryWindow *codexWindow `json:"primary_window"`
	SecondaryWindow *codexWindow `json:"secondary_window"`
}

type codexWindow struct {
	UsedPercent *float64 `json:"used_percent"`
}

// EvaluateCodexUsage parses a chatgpt.com /backend-api/wham/usage JSON body.
// recovered requires:
//   - rate_limit present
//   - allowed != false and limit_reached != true when those fields exist
//   - every present primary/secondary used_percent < threshold
func EvaluateCodexUsage(body []byte, threshold float64) (CodexProbeResult, error) {
	if threshold <= 0 {
		threshold = DefaultUsedPercentThreshold
	}
	var parsed codexUsageBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return CodexProbeResult{}, fmt.Errorf("codex usage json: %w", err)
	}
	if parsed.RateLimit == nil {
		return CodexProbeResult{}, fmt.Errorf("codex usage missing rate_limit")
	}
	rl := parsed.RateLimit
	out := CodexProbeResult{
		Allowed:      rl.Allowed,
		LimitReached: rl.LimitReached,
	}
	if rl.PrimaryWindow != nil {
		out.PrimaryUsedPercent = rl.PrimaryWindow.UsedPercent
	}
	if rl.SecondaryWindow != nil {
		out.SecondaryUsedPercent = rl.SecondaryWindow.UsedPercent
	}

	if rl.Allowed != nil && !*rl.Allowed {
		out.Recovered = false
		return out, nil
	}
	if rl.LimitReached != nil && *rl.LimitReached {
		out.Recovered = false
		return out, nil
	}

	sawWindow := false
	for _, pct := range []*float64{out.PrimaryUsedPercent, out.SecondaryUsedPercent} {
		if pct == nil {
			continue
		}
		sawWindow = true
		if *pct >= threshold {
			out.Recovered = false
			return out, nil
		}
	}
	if !sawWindow {
		return CodexProbeResult{}, fmt.Errorf("codex usage missing used_percent windows")
	}
	out.Recovered = true
	return out, nil
}

// ShouldClearLocalCooldown decides whether to call reset-quota after a live probe.
func ShouldClearLocalCooldown(recovered, localBlock, wakeSucceeded bool) bool {
	if !recovered {
		return false
	}
	return localBlock || wakeSucceeded
}
