package cooldown

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"quota-activation/internal/host"
)

// Options for ClearLocal (POST /v0/management/reset-quota via host.http.do).
type Options struct {
	URL         string
	AuthIndex   string
	BearerToken string
}

// DefaultResetQuotaURL is the in-process CPA management loopback target.
const DefaultResetQuotaURL = "http://127.0.0.1:8317/v0/management/reset-quota"

// ClearLocal clears CPA local model/credential cooldowns for one auth_index.
func ClearLocal(ctx context.Context, client host.Client, opts Options) error {
	if client == nil {
		return fmt.Errorf("cooldown clear: host client required")
	}
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		url = DefaultResetQuotaURL
	}
	authIndex := strings.TrimSpace(opts.AuthIndex)
	if authIndex == "" {
		return fmt.Errorf("cooldown clear: auth_index required")
	}
	token := strings.TrimSpace(opts.BearerToken)
	if token == "" {
		return fmt.Errorf("cooldown clear: management bearer required")
	}
	body, err := json.Marshal(map[string]string{"auth_index": authIndex})
	if err != nil {
		return err
	}
	resp, err := client.HTTPDo(ctx, host.HTTPRequest{
		Method: http.MethodPost,
		URL:    url,
		Headers: host.Header{
			"Authorization": {"Bearer " + token},
			"Content-Type":  {"application/json"},
		},
		Body: body,
	})
	if err != nil {
		return fmt.Errorf("cooldown clear request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(string(resp.Body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("cooldown clear HTTP %d: %s", resp.StatusCode, msg)
	}
	return nil
}
