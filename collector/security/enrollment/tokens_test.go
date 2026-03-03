package enrollment

import (
	"errors"
	"testing"
	"time"
)

func TestIssueAndRedeemSingleUseToken(t *testing.T) {
	m := NewManager()
	now := time.Date(2026, 3, 3, 11, 30, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	token, err := m.IssueToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	if token == "" {
		t.Fatalf("token should not be empty")
	}

	rec, err := m.RedeemToken(token)
	if err != nil {
		t.Fatalf("redeem token failed: %v", err)
	}
	if !rec.Used {
		t.Fatalf("token should be marked used")
	}

	_, err = m.RedeemToken(token)
	if !errors.Is(err, ErrUsedToken) {
		t.Fatalf("expected used-token error, got: %v", err)
	}
}

func TestRedeemExpiredToken(t *testing.T) {
	m := NewManager()
	now := time.Date(2026, 3, 3, 11, 30, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	token, err := m.IssueToken(1 * time.Minute)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	m.now = func() time.Time { return now.Add(2 * time.Minute) }

	_, err = m.RedeemToken(token)
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected expired-token error, got: %v", err)
	}
}

func TestCleanupExpired(t *testing.T) {
	m := NewManager()
	now := time.Date(2026, 3, 3, 11, 30, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	if _, err := m.IssueToken(1 * time.Minute); err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	if _, err := m.IssueToken(10 * time.Minute); err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	m.now = func() time.Time { return now.Add(3 * time.Minute) }
	removed := m.CleanupExpired()
	if removed != 1 {
		t.Fatalf("expected 1 token removed, got %d", removed)
	}
}
