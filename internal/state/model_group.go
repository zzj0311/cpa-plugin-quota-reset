package state

import "strings"

// ModelGroupOf 从 CycleKey 的 provider/group 片段提取模型组。
// 形如 auth:antigravity/gemini:weekly:RFC3339 → gemini；无 suffix 的旧记录返回空。
func ModelGroupOf(rec Record) string {
	key := strings.TrimSpace(rec.CycleKey)
	provider := strings.TrimSpace(rec.Provider)
	if key == "" || provider == "" {
		return ""
	}
	needle := ":" + provider + "/"
	idx := strings.Index(key, needle)
	if idx < 0 {
		return ""
	}
	rest := key[idx+len(needle):]
	group, _, found := strings.Cut(rest, ":")
	if !found {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(group)
}

type latestSuccessQuery struct {
	authID            string
	provider          string
	modelGroup        string
	skipCodexFiveHour bool
	isolateGroup      bool
}

// LatestSuccessForModelGroup 返回同 auth+provider+modelGroup 下最近 success。
// modelGroup 为空只匹配无组记录（Codex / 旧 AG）；非空优先匹配该组，否则回退无组旧记录。
func (s *Store) LatestSuccessForModelGroup(authID string, provider string, modelGroup string) (Record, bool) {
	return s.pickLatestSuccessQuery(latestSuccessQuery{
		authID: authID, provider: provider, modelGroup: modelGroup, isolateGroup: true,
	})
}

// UsableLatestSuccessForModelGroup 与 LatestSuccessForModelGroup 相同，但 Codex 跳过 5h。
func (s *Store) UsableLatestSuccessForModelGroup(authID string, provider string, modelGroup string) (Record, bool) {
	return s.pickLatestSuccessQuery(latestSuccessQuery{
		authID: authID, provider: provider, modelGroup: modelGroup,
		skipCodexFiveHour: true, isolateGroup: true,
	})
}

// LatestSuccessCycleForModelGroup 返回指定模型组最近 success 的 cycle 快照。
func (s *Store) LatestSuccessCycleForModelGroup(authID string, provider string, modelGroup string) (cycleKey string, remaining int64, hasRemaining bool, ok bool) {
	record, found := s.LatestSuccessForModelGroup(authID, provider, modelGroup)
	if !found {
		return "", 0, false, false
	}
	hasRemaining = record.HasRemaining
	if !hasRemaining && record.Remaining != 0 {
		hasRemaining = true
	}
	return record.CycleKey, record.Remaining, hasRemaining, true
}

func (s *Store) pickLatestSuccess(authID string, provider string, skipCodexFiveHour bool) (Record, bool) {
	return s.pickLatestSuccessQuery(latestSuccessQuery{
		authID: authID, provider: provider, skipCodexFiveHour: skipCodexFiveHour,
	})
}

func (s *Store) pickLatestSuccessQuery(q latestSuccessQuery) (Record, bool) {
	if s == nil {
		return Record{}, false
	}
	authID := Redact(strings.TrimSpace(q.authID))
	provider := Redact(strings.TrimSpace(q.provider))
	modelGroup := strings.TrimSpace(q.modelGroup)
	if authID == "" || provider == "" {
		return Record{}, false
	}
	skipFiveHour := q.skipCodexFiveHour && strings.EqualFold(provider, "codex")
	s.mu.RLock()
	defer s.mu.RUnlock()
	var bestExact, bestLegacy Record
	foundExact, foundLegacy := false, false
	for _, record := range s.records {
		if record.AuthID != authID || record.Provider != provider {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(record.LastResult), "success") {
			continue
		}
		if record.Window == "" || record.ResetAt.IsZero() {
			continue
		}
		if skipFiveHour && isFiveHourWindow(record.Window) {
			continue
		}
		if !q.isolateGroup {
			if !foundExact || successNewer(record, bestExact) {
				bestExact = record
				foundExact = true
			}
			continue
		}
		recGroup := ModelGroupOf(record)
		if strings.EqualFold(recGroup, modelGroup) {
			if !foundExact || successNewer(record, bestExact) {
				bestExact = record
				foundExact = true
			}
			continue
		}
		if modelGroup != "" && recGroup == "" {
			if !foundLegacy || successNewer(record, bestLegacy) {
				bestLegacy = record
				foundLegacy = true
			}
		}
	}
	if foundExact {
		return bestExact, true
	}
	return bestLegacy, foundLegacy
}

func successNewer(record Record, best Record) bool {
	return record.ObservedAt.After(best.ObservedAt) || (record.ObservedAt.Equal(best.ObservedAt) && record.ResetAt.After(best.ResetAt))
}
