package runtime

import (
	"context"
	"fmt"

	"quota-activation/internal/activator"
	"quota-activation/internal/config"
	"quota-activation/internal/management"
	"quota-activation/internal/scheduler"
	"quota-activation/internal/session"
	"quota-activation/internal/state"
)

func (r *Runtime) replaceConfig(ctx context.Context, cfg config.Config) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("runtime configure context: %w", err)
	}
	r.mu.Lock()
	prevStore := r.store
	cached := r.snapshotUsableCacheLocked()
	r.mu.Unlock()

	loaded, err := state.Load(ctx, cfg.StatePath)
	if err != nil {
		return err
	}
	if prevStore != nil {
		loaded.MergeUsableSuccessFrom(prevStore)
	}
	for _, rec := range cached {
		loaded.MergeUsableRecord(rec)
	}

	sessions := session.NewManager(session.Options{Now: r.now})
	activation := activator.New(activator.Options{Host: r.host, Sessions: sessions, State: loaded, Config: cfg, Now: r.now, Sleep: r.sleep})
	manager := management.NewHandler(management.Options{
		Activator: activation,
		Host:      r.host,
		Store:     loaded,
		Config:    cfg,
		Now:       r.now,
		OnActivation: func(result activator.Result, err error) {
			r.snapshotActivation(runHistoryTriggerManual, result, err)
			if result.Success && result.Status == activator.StatusSuccess {
				r.rememberUsableSuccess(usableRecordFromResult(result))
			}
			if err == nil && result.Success && result.Status == activator.StatusSuccess {
				r.clearSchedulerMissCooldown(result.AuthID)
			}
		},
		RunHistory: func() any {
			return r.currentRunHistory()
		},
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shutdown {
		return ErrShutdown
	}
	r.config = cfg
	r.sessions = sessions
	r.store = loaded
	r.seedUsableCacheFromStoreLocked(loaded)
	r.activator = activation
	r.management = manager
	r.picker = scheduler.NewPicker(sessions)
	if !cfg.AutoActivate {
		r.stopAutoScannerLocked()
	} else {
		r.startAutoScannerLocked(cfg.ScanInterval)
	}
	return nil
}
