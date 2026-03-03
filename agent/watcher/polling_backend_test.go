package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPollingBackendDetectsWriteAndRemove(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "app.log")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatalf("write initial file failed: %v", err)
	}

	b := newPollingBackend(10 * time.Millisecond)
	defer b.Close()
	if err := b.Add(path); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatalf("write update failed: %v", err)
	}
	if _, ok := waitForEventOp(b.Events(), OpWrite, 2*time.Second); !ok {
		t.Fatalf("expected write event")
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove file failed: %v", err)
	}
	if _, ok := waitForEventOp(b.Events(), OpRemove, 2*time.Second); !ok {
		t.Fatalf("expected remove event")
	}
}

func TestPollingBackendDetectsCreateForMissingPath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "new.log")

	b := newPollingBackend(10 * time.Millisecond)
	defer b.Close()
	if err := b.Add(path); err != nil {
		t.Fatalf("add missing path failed: %v", err)
	}

	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatalf("write create file failed: %v", err)
	}
	if _, ok := waitForEventOp(b.Events(), OpCreate, 2*time.Second); !ok {
		t.Fatalf("expected create event")
	}
}

func TestAdaptivePollingSkipsUntilTierIntervalReached(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "adaptive.log")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatalf("write initial file failed: %v", err)
	}

	policy := AdaptivePollingPolicy{
		HotInterval:  10 * time.Millisecond,
		WarmInterval: 100 * time.Millisecond,
		ColdInterval: time.Second,
		WarmAfter:    500 * time.Millisecond,
		ColdAfter:    2 * time.Second,
	}
	b := newAdaptivePollingBackend(policy)
	defer b.Close()
	b.ticker.Stop()

	base := time.Unix(200, 0).UTC()
	b.now = func() time.Time { return base }

	if err := b.Add(path); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatalf("write update failed: %v", err)
	}

	b.mu.Lock()
	tracked := b.paths[path]
	tracked.lastActivity = base.Add(-3 * time.Second)
	tracked.lastScanned = base.Add(-200 * time.Millisecond)
	b.paths[path] = tracked
	b.mu.Unlock()

	b.scanOnce()
	select {
	case ev := <-b.Events():
		t.Fatalf("did not expect event before cold interval elapsed: %+v", ev)
	default:
	}

	b.mu.Lock()
	tracked = b.paths[path]
	tracked.lastScanned = base.Add(-2 * time.Second)
	b.paths[path] = tracked
	b.mu.Unlock()

	b.scanOnce()
	if _, ok := waitForEventOp(b.Events(), OpWrite, time.Second); !ok {
		t.Fatalf("expected write event after cold interval elapsed")
	}
}

func waitForEventOp(ch <-chan Event, op Op, timeout time.Duration) (Event, bool) {
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return Event{}, false
			}
			if ev.Op&op != 0 {
				return ev, true
			}
		case <-deadline:
			return Event{}, false
		}
	}
}
