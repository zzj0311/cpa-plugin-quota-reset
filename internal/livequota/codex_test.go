package livequota

import (
	"testing"
)

func TestCodexRecovered_BothUnderThreshold(t *testing.T) {
	body := []byte(`{
		"rate_limit": {
			"allowed": true,
			"limit_reached": false,
			"primary_window": {"used_percent": 90, "limit_window_seconds": 18000},
			"secondary_window": {"used_percent": 68, "limit_window_seconds": 604800}
		}
	}`)
	got, err := EvaluateCodexUsage(body, 95)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !got.Recovered {
		t.Fatalf("expected recovered, got %+v", got)
	}
}

func TestCodexRecovered_SecondaryAtOrAboveThreshold(t *testing.T) {
	body := []byte(`{
		"rate_limit": {
			"primary_window": {"used_percent": 10},
			"secondary_window": {"used_percent": 95}
		}
	}`)
	got, err := EvaluateCodexUsage(body, 95)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Recovered {
		t.Fatalf("expected not recovered at 95%%, got %+v", got)
	}
}

func TestCodexRecovered_AllowedFalseWins(t *testing.T) {
	body := []byte(`{
		"rate_limit": {
			"allowed": false,
			"limit_reached": true,
			"primary_window": {"used_percent": 1},
			"secondary_window": {"used_percent": 1}
		}
	}`)
	got, err := EvaluateCodexUsage(body, 95)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Recovered {
		t.Fatalf("expected not recovered when allowed=false")
	}
}

func TestCodexRecovered_MissingSecondaryUsesPrimaryOnly(t *testing.T) {
	body := []byte(`{"rate_limit":{"primary_window":{"used_percent": 50}}}`)
	got, err := EvaluateCodexUsage(body, 95)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !got.Recovered {
		t.Fatalf("expected recovered with only primary under threshold")
	}
}

func TestShouldClearLocalCooldown(t *testing.T) {
	cases := []struct {
		name      string
		recovered bool
		localBlock bool
		wakeOK    bool
		want      bool
	}{
		{"recovered+blocked", true, true, false, true},
		{"recovered+wake", true, false, true, true},
		{"recovered+neither", true, false, false, false},
		{"not_recovered+blocked", false, true, false, false},
		{"not_recovered+wake", false, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldClearLocalCooldown(tc.recovered, tc.localBlock, tc.wakeOK)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
