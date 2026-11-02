package detector

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func decodePayload(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("quota json: %w", ErrMalformedQuota)
	}
	return nil
}
