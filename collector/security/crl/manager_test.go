package crl

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRevokeUnrevokeSnapshotAndVersion(t *testing.T) {
	m := NewManager()
	now := time.Date(2026, 3, 3, 15, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	if err := m.Revoke("0x03e9", "manual block"); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if !m.IsRevoked("1001") {
		t.Fatalf("expected decimal serial to be revoked after canonicalization")
	}

	// Schedule one future revocation and verify effective-time behavior.
	if err := m.RevokeAfterWithReason("1002", now.Add(time.Minute), "overlap expiry"); err != nil {
		t.Fatalf("revoke-after failed: %v", err)
	}
	if m.IsRevoked("1002") {
		t.Fatalf("serial should not be revoked before effective time")
	}
	now = now.Add(2 * time.Minute)
	if !m.IsRevoked("1002") {
		t.Fatalf("serial should be revoked after effective time")
	}

	snap := m.Snapshot()
	if snap.Version != 2 || len(snap.Entries) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if snap.Entries[0].Serial != "1001" || snap.Entries[1].Serial != "1002" {
		t.Fatalf("expected sorted serial entries, got %+v", snap.Entries)
	}

	removed, err := m.Unrevoke("0x03e9")
	if err != nil || !removed {
		t.Fatalf("unrevoke failed removed=%v err=%v", removed, err)
	}
	snap = m.Snapshot()
	if snap.Version != 3 || len(snap.Entries) != 1 || snap.Entries[0].Serial != "1002" {
		t.Fatalf("unexpected snapshot after unrevoke: %+v", snap)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crl", "snapshot.json")
	m := NewManager()
	now := time.Date(2026, 3, 3, 15, 5, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	if err := m.Revoke("1005", "test"); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if err := m.Save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := NewManager()
	loaded.now = func() time.Time { return now }
	if err := loaded.Load(path); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !loaded.IsRevoked("1005") {
		t.Fatalf("expected loaded serial to be revoked")
	}
	if loaded.Snapshot().Version != 1 {
		t.Fatalf("expected loaded snapshot version 1")
	}
}

func TestCanonicalSerial(t *testing.T) {
	serial, err := CanonicalSerial("0x0A")
	if err != nil || serial != "10" {
		t.Fatalf("expected hex canonicalization to 10, got %q err=%v", serial, err)
	}
	serial, err = CanonicalSerial("00:0a")
	if err != nil || serial != "10" {
		t.Fatalf("expected colon-separated hex canonicalization to 10, got %q err=%v", serial, err)
	}
	if _, err := CanonicalSerial("bad-serial"); err == nil {
		t.Fatalf("expected invalid serial error")
	}
}
