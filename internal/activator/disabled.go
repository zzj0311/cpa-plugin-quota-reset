package activator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"quota-activation/internal/host"
)

func (a *Activator) enableIfNeeded(ctx context.Context, request Request) (bool, error) {
	if !request.Disabled {
		return false, nil
	}
	files, err := a.host.ListAuthFiles(ctx)
	if err != nil {
		return false, fmt.Errorf("list auth files: %w", safeError(err))
	}
	for _, file := range files {
		updated, ok, err := enableAuthFile(file, request.AuthID)
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		if err := a.host.SaveAuthFile(ctx, file.Name, updated); err != nil {
			return false, fmt.Errorf("enable auth file: %w", safeError(err))
		}
		return true, nil
	}
	return false, ErrAuthFileNotFound
}

func enableAuthFile(file host.AuthFile, authID string) ([]byte, bool, error) {
	if strings.TrimSpace(file.Name) == strings.TrimSpace(authID) {
		return withDisabled(file.Data, false)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(file.Data, &document); err != nil {
		return nil, false, nil
	}
	if !authFileMatches(document, authID) {
		return nil, false, nil
	}
	return withDisabled(file.Data, false)
}

func authFileMatches(document map[string]json.RawMessage, authID string) bool {
	for _, key := range []string{"auth_id", "authID", "id", "name"} {
		raw, ok := document[key]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err == nil && strings.TrimSpace(value) == strings.TrimSpace(authID) {
			return true
		}
	}
	return false
}

func withDisabled(data []byte, disabled bool) ([]byte, bool, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, false, fmt.Errorf("decode auth file: %w", safeError(err))
	}
	value, err := json.Marshal(disabled)
	if err != nil {
		return nil, false, fmt.Errorf("encode disabled flag: %w", safeError(err))
	}
	document["disabled"] = value
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, false, fmt.Errorf("encode auth file: %w", safeError(err))
	}
	return encoded, true, nil
}
