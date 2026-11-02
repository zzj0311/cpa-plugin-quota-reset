package runtime

import (
	"encoding/json"
	"fmt"

	"quota-activation/internal/scheduler"
)

// SchedulerResponse 是 scheduler.pick 返回给 CPA 的保守调度决策。
type SchedulerResponse struct {
	Handled bool   `json:"Handled"`
	AuthID  string `json:"AuthID,omitempty"`
	Reason  string `json:"Reason"`
}

func (r *Runtime) pickSchedule(raw []byte) []byte {
	picker, autoActivate, err := r.schedulerPicker()
	if err != nil {
		return failure(err)
	}
	request, err := decodeSchedulerRequest(raw)
	if err != nil {
		return failure(err)
	}
	request.AutoActivate = autoActivate
	decision := picker.Pick(request)
	return envelopeResult(SchedulerResponse{Handled: decision.Handled, AuthID: decision.AuthID, Reason: decision.Reason}, nil)
}

func (r *Runtime) schedulerPicker() (scheduler.Picker, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shutdown {
		return scheduler.Picker{}, false, ErrShutdown
	}
	return r.picker, r.config.AutoActivate, nil
}

func decodeSchedulerRequest(raw []byte) (scheduler.PickRequest, error) {
	if len(raw) == 0 {
		return scheduler.PickRequest{}, nil
	}
	var wire schedulerRequestWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return scheduler.PickRequest{}, fmt.Errorf("%w: decode scheduler request: %v", ErrInvalidRequest, err)
	}
	return wire.toPickRequest(), nil
}

type schedulerRequestWire struct {
	Candidates []schedulerCandidateWire `json:"Candidates"`
	Headers    map[string][]string      `json:"Headers"`
	Metadata   map[string]any           `json:"Metadata"`
	Options    schedulerOptionsWire     `json:"Options"`
}

type schedulerOptionsWire struct {
	Headers  map[string][]string `json:"Headers"`
	Metadata map[string]any      `json:"Metadata"`
}

type schedulerCandidateWire struct {
	ID             string `json:"ID"`
	IDLower        string `json:"id"`
	AuthID         string `json:"AuthID"`
	AuthIDLower    string `json:"auth_id"`
	AuthIndex      string `json:"AuthIndex"`
	AuthIndexLower string `json:"auth_index"`
}

func (w schedulerRequestWire) toPickRequest() scheduler.PickRequest {
	candidates := make([]scheduler.Candidate, 0, len(w.Candidates))
	for _, candidate := range w.Candidates {
		candidates = append(candidates, scheduler.Candidate{AuthID: firstNonEmpty(
			candidate.ID,
			candidate.IDLower,
			candidate.AuthID,
			candidate.AuthIDLower,
			candidate.AuthIndex,
			candidate.AuthIndexLower,
		)})
	}
	headers := w.Headers
	if len(w.Options.Headers) > 0 {
		headers = w.Options.Headers
	}
	metadata := normalizeStringValues(w.Metadata)
	if optionMetadata := normalizeStringValues(w.Options.Metadata); len(optionMetadata) > 0 {
		metadata = optionMetadata
	}
	return scheduler.PickRequest{Candidates: candidates, Headers: headers, Metadata: metadata}
}

func normalizeStringValues(values map[string]any) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string][]string, len(values))
	for key, raw := range values {
		switch value := raw.(type) {
		case string:
			normalized[key] = []string{value}
		case []string:
			if len(value) > 0 {
				normalized[key] = append([]string(nil), value...)
			}
		case []any:
			items := make([]string, 0, len(value))
			for _, item := range value {
				if text, ok := item.(string); ok {
					items = append(items, text)
				}
			}
			if len(items) > 0 {
				normalized[key] = items
			}
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
