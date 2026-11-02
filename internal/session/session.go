package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	// ErrEmptyNonce 表示调用方没有提供一次性会话 nonce。
	ErrEmptyNonce = errors.New("session: empty nonce")
	// ErrSessionNotFound 表示 nonce 未匹配任何有效会话。
	ErrSessionNotFound = errors.New("session: not found")
	// ErrSessionExpired 表示 nonce 匹配的会话已经超过 TTL。
	ErrSessionExpired = errors.New("session: expired")
)

const defaultTTL = 10 * time.Minute

// NonceGenerator 生成一次性激活会话 nonce。
type NonceGenerator func() (string, error)

// Options 定义一次性激活会话管理器的可注入依赖。
type Options struct {
	TTL      time.Duration
	Now      func() time.Time
	Generate NonceGenerator
}

// Target 描述一次性激活会话绑定的目标凭证窗口。
type Target struct {
	AuthID   string
	Provider string
	Window   string
}

// Session 是一次性激活流程在内存中的可查询会话。
type Session struct {
	Target    Target
	Nonce     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Manager 管理仅能消费一次的激活会话。
type Manager struct {
	mu       sync.Mutex
	ttl      time.Duration
	now      func() time.Time
	generate NonceGenerator
	sessions map[string]Session
}

// NewManager 构造带安全随机默认 nonce 的会话管理器。
func NewManager(options Options) *Manager {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	generate := options.Generate
	if generate == nil {
		generate = secureNonce
	}
	return &Manager{ttl: ttl, now: now, generate: generate, sessions: map[string]Session{}}
}

// Create 生成唯一 nonce，并将其绑定到目标 AuthID 与 provider/window。
func (m *Manager) Create(target Target) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	createdAt := m.now().UTC()
	for range 8 {
		nonce, err := m.generate()
		if err != nil {
			return Session{}, fmt.Errorf("generate session nonce: %w", err)
		}
		nonce = strings.TrimSpace(nonce)
		if nonce == "" {
			continue
		}
		if _, exists := m.sessions[nonce]; exists {
			continue
		}
		session := Session{Target: target, Nonce: nonce, CreatedAt: createdAt, ExpiresAt: createdAt.Add(m.ttl)}
		m.sessions[nonce] = session
		return session, nil
	}
	return Session{}, fmt.Errorf("generate session nonce: %w", ErrSessionNotFound)
}

// Lookup 返回 nonce 对应的未过期会话，但不消费它。
func (m *Manager) Lookup(nonce string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lookupLocked(nonce)
}

// Consume 返回 nonce 对应的未过期会话，并立即删除该 nonce。
func (m *Manager) Consume(nonce string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, err := m.lookupLocked(nonce)
	if err != nil {
		return Session{}, err
	}
	delete(m.sessions, strings.TrimSpace(nonce))
	return session, nil
}

func (m *Manager) lookupLocked(nonce string) (Session, error) {
	key := strings.TrimSpace(nonce)
	if key == "" {
		return Session{}, ErrEmptyNonce
	}
	session, ok := m.sessions[key]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if !m.now().UTC().Before(session.ExpiresAt) {
		delete(m.sessions, key)
		return Session{}, ErrSessionExpired
	}
	return session, nil
}

func secureNonce() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("read secure random: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
