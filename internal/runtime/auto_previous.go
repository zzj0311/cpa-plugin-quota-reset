package runtime

import (
	"quota-activation/internal/detector"
	"quota-activation/internal/state"
)

func previousStateFromStore(store *state.Store, authID string, provider detector.Provider) detector.PreviousState {
	return previousStateFromStoreForModelGroup(store, authID, provider, detector.ModelGroupNone)
}

func previousStateFromStoreForModelGroup(store *state.Store, authID string, provider detector.Provider, modelGroup detector.ModelGroup) detector.PreviousState {
	if store == nil {
		return detector.PreviousState{}
	}
	cycleKey, remaining, hasRemaining, ok := store.LatestSuccessCycleForModelGroup(authID, string(provider), string(modelGroup))
	if !ok {
		return detector.PreviousState{}
	}
	if !hasRemaining && remaining != 0 {
		hasRemaining = true
	}
	return detector.PreviousState{
		CycleKey: cycleKey, Remaining: remaining, HasRemaining: hasRemaining,
	}
}
