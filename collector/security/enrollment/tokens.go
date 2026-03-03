package enrollment

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid enrollment token")
	ErrExpiredToken = errors.New("expired enrollment token")
	ErrUsedToken    = errors.New("enrollment token already redeemed")
	ErrInvalidTTL   = errors.New("token ttl must be positive")
)

// Record captures token lifecycle state for audit and validation.
type Record struct {
	IssuedAt   time.Time
	ExpiresAt  time.Time
	RedeemedAt time.Time
	Used       bool
}

// Manager issues and validates single-use enrollment tokens.
type Manager struct {
	mu      sync.Mutex
	now     func() time.Time
	records map[[32]byte]Record
}

func NewManager() *Manager {
	return &Manager{
		now:     time.Now,
		records: make(map[[32]byte]Record),
	}
}

// IssueToken creates a single-use token with an expiry duration.
func (m *Manager) IssueToken(ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", ErrInvalidTTL
	}
	raw, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := m.now().UTC()
	h := hashToken(raw)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[h] = Record{
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
		Used:      false,
	}

	return raw, nil
}

// RedeemToken validates and marks a token as consumed.
func (m *Manager) RedeemToken(token string) (Record, error) {
	if token == "" {
		return Record{}, ErrInvalidToken
	}
	now := m.now().UTC()
	h := hashToken(token)

	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.records[h]
	if !ok {
		return Record{}, ErrInvalidToken
	}
	if rec.Used {
		return Record{}, ErrUsedToken
	}
	if now.After(rec.ExpiresAt) {
		return Record{}, ErrExpiredToken
	}

	rec.Used = true
	rec.RedeemedAt = now
	m.records[h] = rec
	return rec, nil
}

// CleanupExpired removes expired, unused tokens to keep map size bounded.
func (m *Manager) CleanupExpired() int {
	now := m.now().UTC()
	removed := 0

	m.mu.Lock()
	defer m.mu.Unlock()

	for k, rec := range m.records {
		if !rec.Used && now.After(rec.ExpiresAt) {
			delete(m.records, k)
			removed++
		}
	}
	return removed
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}
