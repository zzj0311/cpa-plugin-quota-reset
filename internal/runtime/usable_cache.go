package runtime

import (
	"quota-activation/internal/activator"
	"quota-activation/internal/state"
)

type usableAuthKey struct {
	AuthID     string
	Provider   string
	ModelGroup string
}

func (r *Runtime) rememberUsableSuccess(rec state.Record) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rememberUsableSuccessLocked(rec)
}

func (r *Runtime) rememberUsableSuccessLocked(rec state.Record) {
	if r == nil || !usableSuccessForCache(rec) {
		return
	}
	if r.lastUsableSuccess == nil {
		r.lastUsableSuccess = make(map[usableAuthKey]state.Record)
	}
	r.lastUsableSuccess[usableCacheKey(rec)] = rec
}

func (r *Runtime) snapshotUsableCacheLocked() []state.Record {
	if r == nil || len(r.lastUsableSuccess) == 0 {
		return nil
	}
	out := make([]state.Record, 0, len(r.lastUsableSuccess))
	for _, rec := range r.lastUsableSuccess {
		out = append(out, rec)
	}
	return out
}

func (r *Runtime) seedUsableCacheFromStoreLocked(store *state.Store) {
	if r == nil {
		return
	}
	if r.lastUsableSuccess == nil {
		r.lastUsableSuccess = make(map[usableAuthKey]state.Record)
	}
	if store == nil {
		return
	}
	for _, rec := range store.SnapshotUsableSuccess() {
		r.rememberUsableSuccessLocked(rec)
	}
}

func usableSuccessForCache(rec state.Record) bool {
	probe := state.NewStore("")
	probe.MergeUsableRecord(rec)
	_, ok := probe.UsableLatestSuccessForModelGroup(rec.AuthID, rec.Provider, state.ModelGroupOf(rec))
	return ok
}

func usableCacheKey(rec state.Record) usableAuthKey {
	return usableAuthKey{AuthID: rec.AuthID, Provider: rec.Provider, ModelGroup: state.ModelGroupOf(rec)}
}

func usableRecordFromResult(result activator.Result) state.Record {
	return state.Record{
		AuthID:       result.AuthID,
		Provider:     result.Provider,
		Window:       result.Window,
		CycleKey:     result.CycleKey,
		ObservedAt:   result.ObservedAt,
		ResetAt:      result.ResetAt,
		LastResult:   string(activator.StatusSuccess),
		LastError:    result.LastError,
		Remaining:    result.Remaining,
		HasRemaining: result.HasRemaining,
	}
}
