package cursor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSetGetReloadDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cursors.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	now := time.Date(2026, 3, 3, 12, 5, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	if err := s.Set("source-1", "app.log", "inode:1", 123); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	cur, ok := s.Get("source-1", "app.log")
	if !ok || cur.Offset != 123 {
		t.Fatalf("unexpected cursor: %+v ok=%v", cur, ok)
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload store failed: %v", err)
	}
	cur2, ok := s2.Get("source-1", "app.log")
	if !ok || cur2.Offset != 123 {
		t.Fatalf("unexpected reloaded cursor: %+v ok=%v", cur2, ok)
	}

	if err := s2.Delete("source-1", "app.log"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, ok := s2.Get("source-1", "app.log"); ok {
		t.Fatalf("cursor should be deleted")
	}
}

func TestStoreCorruptionFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cursors.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file failed: %v", err)
	}

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store with corrupt file failed: %v", err)
	}
	if got := len(s.All()); got != 0 {
		t.Fatalf("expected empty store after corruption fallback, got %d", got)
	}

	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one corrupt backup file, got %d", len(matches))
	}
}
