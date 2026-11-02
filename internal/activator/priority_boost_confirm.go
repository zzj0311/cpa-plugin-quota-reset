package activator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"quota-activation/internal/host"
)

const (
	boostConfirmAttempts = 3
	boostConfirmDelay    = 20 * time.Millisecond
)

// confirmBoostedUniqueTop 在 Save 后通过 GetRuntimeAuthFile（必要时辅以 List）确认目标
// 在同 provider 中为唯一最高 priority；失败则返回纯中文错误，调用方不得 ModelExecute。
func (a *Activator) confirmBoostedUniqueTop(ctx context.Context, target host.AuthFile, provider, name string, wantPriority int) error {
	authIndex := strings.TrimSpace(firstNonBlank(target.AuthIndex, target.ID, target.Name, name))
	display := strings.TrimSpace(firstNonBlank(name, authIndex))
	var lastErr error
	for attempt := 0; attempt < boostConfirmAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("优先级确认已取消：%w", ctx.Err())
			case <-time.After(boostConfirmDelay):
			}
		}
		confirmed, err := a.readRuntimePriority(ctx, authIndex, name)
		if err != nil {
			lastErr = err
			continue
		}
		if confirmed < wantPriority {
			lastErr = fmt.Errorf("优先级提升后，运行时仍未确认目标为最高优先级：%s", display)
			continue
		}
		files, listErr := a.host.ListAuthFiles(ctx)
		if listErr != nil {
			lastErr = fmt.Errorf("确认优先级时列举凭证失败：%w", safeHostError(listErr))
			continue
		}
		if uniqueTopInProvider(files, provider, authIndex, name, confirmed) {
			return nil
		}
		lastErr = fmt.Errorf("优先级提升后，运行时仍未确认目标为最高优先级：%s", display)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("优先级提升后，运行时仍未确认目标为最高优先级：%s", display)
	}
	return lastErr
}

func (a *Activator) readRuntimePriority(ctx context.Context, authIndex, name string) (int, error) {
	if a == nil || a.host == nil {
		return 0, fmt.Errorf("优先级确认失败：宿主不可用")
	}
	keys := []string{authIndex, name}
	var lastErr error
	display := strings.TrimSpace(firstNonBlank(name, authIndex))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		file, err := a.host.GetRuntimeAuthFile(ctx, key)
		if err != nil {
			lastErr = fmt.Errorf("读取运行时凭证失败：%s", display)
			continue
		}
		if p := priorityOfAuthFile(file); p != 0 || len(file.Data) > 0 {
			if p == 0 {
				p = priorityFromJSON(file.Data, 0)
			}
			return p, nil
		}
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("优先级提升后，运行时仍未确认目标为最高优先级：%s", display)
}

func uniqueTopInProvider(files []host.AuthFile, provider, authIndex, name string, targetPriority int) bool {
	wantIDs := map[string]struct{}{}
	for _, id := range []string{authIndex, name} {
		id = strings.TrimSpace(id)
		if id != "" {
			wantIDs[id] = struct{}{}
		}
	}
	peerAtOrAbove := 0
	targetSeen := false
	for _, file := range files {
		if strings.ToLower(strings.TrimSpace(firstNonBlank(file.Provider, file.Type))) != provider {
			continue
		}
		p := priorityOfAuthFile(file)
		isTarget := authFileIsTarget(file, wantIDs)
		if isTarget {
			targetSeen = true
			if p < targetPriority {
				p = targetPriority
			}
		}
		if p > targetPriority {
			return false
		}
		if p == targetPriority {
			peerAtOrAbove++
			if !isTarget {
				return false
			}
		}
	}
	return targetSeen && peerAtOrAbove >= 1
}

func authFileIsTarget(file host.AuthFile, wantIDs map[string]struct{}) bool {
	for _, id := range []string{file.ID, file.Name, file.AuthIndex} {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := wantIDs[id]; ok {
			return true
		}
	}
	return false
}
