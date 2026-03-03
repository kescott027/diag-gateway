package crl

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry is one certificate-serial revocation record.
type Entry struct {
	Serial      string    `json:"serial"`
	EffectiveAt time.Time `json:"effective_at"`
	Reason      string    `json:"reason,omitempty"`
}

// Snapshot is a deterministic CRL state view.
type Snapshot struct {
	Version   uint64    `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Entries   []Entry   `json:"entries"`
}

// Manager stores revocation entries with versioned state snapshots.
type Manager struct {
	mu      sync.RWMutex
	entries map[string]Entry
	version uint64
	updated time.Time
	now     func() time.Time
}

func NewManager() *Manager {
	return &Manager{
		entries: make(map[string]Entry),
		now:     time.Now,
	}
}

// Revoke marks a serial revoked immediately.
func (m *Manager) Revoke(serial, reason string) error {
	return m.RevokeAfterWithReason(serial, m.now().UTC(), reason)
}

// RevokeAfter keeps compatibility with admission/revocation serial checker integrations.
func (m *Manager) RevokeAfter(serial string, at time.Time) {
	_ = m.RevokeAfterWithReason(serial, at, "")
}

// RevokeAfterWithReason marks a serial revoked at a specific time.
func (m *Manager) RevokeAfterWithReason(serial string, at time.Time, reason string) error {
	canonical, err := CanonicalSerial(serial)
	if err != nil {
		return err
	}
	if at.IsZero() {
		at = m.now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[canonical] = Entry{
		Serial:      canonical,
		EffectiveAt: at.UTC(),
		Reason:      strings.TrimSpace(reason),
	}
	m.version++
	m.updated = m.now().UTC()
	return nil
}

// Unrevoke removes a serial from CRL state.
func (m *Manager) Unrevoke(serial string) (bool, error) {
	canonical, err := CanonicalSerial(serial)
	if err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[canonical]; !ok {
		return false, nil
	}
	delete(m.entries, canonical)
	m.version++
	m.updated = m.now().UTC()
	return true, nil
}

// IsRevoked checks whether serial is currently effective in the CRL.
func (m *Manager) IsRevoked(serial string) bool {
	canonical, err := CanonicalSerial(serial)
	if err != nil {
		return false
	}
	m.mu.RLock()
	entry, ok := m.entries[canonical]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return !m.now().UTC().Before(entry.EffectiveAt)
}

// Snapshot returns deterministic sorted CRL state.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	out := Snapshot{
		Version:   m.version,
		UpdatedAt: m.updated,
		Entries:   make([]Entry, 0, len(m.entries)),
	}
	for _, entry := range m.entries {
		out.Entries = append(out.Entries, entry)
	}
	m.mu.RUnlock()
	sort.Slice(out.Entries, func(i, j int) bool {
		return out.Entries[i].Serial < out.Entries[j].Serial
	})
	return out
}

// Save writes an atomic JSON snapshot to disk.
func (m *Manager) Save(path string) error {
	snap := m.Snapshot()
	blob, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal crl snapshot: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create crl dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return fmt.Errorf("write crl temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("promote crl file: %w", err)
	}
	return nil
}

// Load replaces CRL state from a JSON snapshot file.
func (m *Manager) Load(path string) error {
	blob, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var snap Snapshot
	if err := json.Unmarshal(blob, &snap); err != nil {
		return fmt.Errorf("decode crl snapshot: %w", err)
	}

	entries := make(map[string]Entry, len(snap.Entries))
	for _, entry := range snap.Entries {
		canonical, err := CanonicalSerial(entry.Serial)
		if err != nil {
			return fmt.Errorf("invalid serial in crl snapshot: %w", err)
		}
		entry.Serial = canonical
		entry.EffectiveAt = entry.EffectiveAt.UTC()
		entries[canonical] = entry
	}

	m.mu.Lock()
	m.entries = entries
	m.version = snap.Version
	m.updated = snap.UpdatedAt.UTC()
	m.mu.Unlock()
	return nil
}

// CanonicalSerial normalizes decimal/hex serial representations.
func CanonicalSerial(serial string) (string, error) {
	raw := strings.TrimSpace(strings.ToLower(serial))
	if raw == "" {
		return "", fmt.Errorf("serial is required")
	}
	raw = strings.ReplaceAll(raw, ":", "")
	base := 10
	if strings.HasPrefix(raw, "0x") {
		raw = strings.TrimPrefix(raw, "0x")
		base = 16
	}
	if base != 16 {
		for _, ch := range raw {
			if ch >= 'a' && ch <= 'f' {
				base = 16
				break
			}
		}
	}
	for _, ch := range raw {
		if ch < '0' || (ch > '9' && (ch < 'a' || ch > 'f')) {
			return "", fmt.Errorf("invalid serial %q", serial)
		}
	}
	n := new(big.Int)
	if _, ok := n.SetString(raw, base); !ok || n.Sign() < 0 {
		return "", fmt.Errorf("invalid serial %q", serial)
	}
	return n.String(), nil
}
