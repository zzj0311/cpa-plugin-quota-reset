package scheduler

import (
	"errors"
	"strings"

	"quota-activation/internal/session"
)

const (
	// NonceHeaderName 是唤醒请求携带一次性会话 nonce 的 HTTP header 名称。
	NonceHeaderName = "X-Quota-Activation-Nonce"
	// NonceMetadataKey 是唤醒请求携带一次性会话 nonce 的 metadata 键名。
	NonceMetadataKey = "quota_activation_nonce"

	// ReasonNonceMatched 表示 nonce 有效、目标候选存在且本次调度已接管。
	ReasonNonceMatched = "nonce_matched"
	// ReasonNonceMissing 表示请求未携带非空唤醒 nonce，调度应交回宿主。
	ReasonNonceMissing = "nonce_missing"
	// ReasonNonceUnknown 表示 nonce 不存在或已被消费，调度应交回宿主。
	ReasonNonceUnknown = "nonce_unknown"
	// ReasonNonceExpired 表示 nonce 已过 TTL，调度应交回宿主。
	ReasonNonceExpired = "nonce_expired"
	// ReasonTargetCandidateMissing 表示会话目标 AuthID 不在候选列表中，调度应交回宿主。
	ReasonTargetCandidateMissing = "target_candidate_missing"
	// ReasonSessionUnavailable 表示调度器缺少会话管理器，调度应交回宿主。
	ReasonSessionUnavailable = "session_unavailable"
)

// Candidate 描述宿主传入 scheduler.pick 的候选凭证。
type Candidate struct {
	AuthID string
}

// PickRequest 描述 scheduler.pick 所需的内部判定输入。
type PickRequest struct {
	Candidates   []Candidate
	Headers      map[string][]string
	Metadata     map[string][]string
	AutoActivate bool
}

// PickDecision 描述 scheduler.pick 是否接管宿主调度以及选中的目标凭证。
type PickDecision struct {
	Handled bool
	AuthID  string
	Reason  string
}

// Picker 使用一次性 session nonce 精确选择配额唤醒目标凭证。
type Picker struct {
	manager *session.Manager
}

// NewPicker 构造仅依赖 session.Manager 的调度判定器。
func NewPicker(manager *session.Manager) Picker {
	return Picker{manager: manager}
}

// Pick 在 nonce 有效且目标 AuthID 存在于候选列表时接管调度。
func (p Picker) Pick(request PickRequest) PickDecision {
	if p.manager == nil {
		return delegateToHost(ReasonSessionUnavailable)
	}
	nonce, ok := nonceFromRequest(request)
	if !ok {
		return delegateToHost(ReasonNonceMissing)
	}
	found, err := p.manager.Lookup(nonce)
	if err != nil {
		return delegateToHost(reasonForSessionError(err))
	}
	if !hasCandidate(request.Candidates, found.Target.AuthID) {
		return delegateToHost(ReasonTargetCandidateMissing)
	}
	if _, err := p.manager.Consume(nonce); err != nil {
		return delegateToHost(reasonForSessionError(err))
	}
	return PickDecision{Handled: true, AuthID: found.Target.AuthID, Reason: ReasonNonceMatched}
}

func nonceFromRequest(request PickRequest) (string, bool) {
	if nonce, ok := firstNonEmptyValue(request.Metadata, NonceMetadataKey, false); ok {
		return nonce, true
	}
	return firstNonEmptyValue(request.Headers, NonceHeaderName, true)
}

func firstNonEmptyValue(values map[string][]string, key string, foldKey bool) (string, bool) {
	for candidateKey, candidateValues := range values {
		if !keysEqual(candidateKey, key, foldKey) {
			continue
		}
		for _, value := range candidateValues {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed, true
			}
		}
	}
	return "", false
}

func keysEqual(left string, right string, fold bool) bool {
	if fold {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func hasCandidate(candidates []Candidate, authID string) bool {
	for _, candidate := range candidates {
		if candidate.AuthID == authID {
			return true
		}
	}
	return false
}

func reasonForSessionError(err error) string {
	if errors.Is(err, session.ErrSessionExpired) {
		return ReasonNonceExpired
	}
	return ReasonNonceUnknown
}

func delegateToHost(reason string) PickDecision {
	return PickDecision{Reason: reason}
}
