package activator

import (
	"quota-activation/internal/state"
)

type usableAuthKey struct {
	AuthID     string
	Provider   string
	ModelGroup string
}

func (a *Activator) rememberUsableSuccess(rec state.Record) {
	if a == nil || !usableSuccessForCache(rec) {
		return
	}
	a.usableMu.Lock()
	defer a.usableMu.Unlock()
	if a.lastUsableSuccess == nil {
		a.lastUsableSuccess = make(map[usableAuthKey]state.Record)
	}
	a.lastUsableSuccess[usableCacheKey(rec)] = rec
}

func (a *Activator) hardSkipStore() *state.Store {
	probe := state.NewStore("")
	if a == nil {
		return probe
	}
	if a.store != nil {
		probe.MergeUsableSuccessFrom(a.store)
	}
	a.usableMu.Lock()
	cached := make([]state.Record, 0, len(a.lastUsableSuccess))
	for _, rec := range a.lastUsableSuccess {
		cached = append(cached, rec)
	}
	a.usableMu.Unlock()
	for _, rec := range cached {
		probe.MergeUsableRecord(rec)
	}
	return probe
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
