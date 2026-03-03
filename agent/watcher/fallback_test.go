package watcher

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type backendFake struct {
	addErr    error
	removeErr error
	closeErr  error
	closed    bool

	mu      sync.Mutex
	added   []string
	removed []string

	events chan Event
	errors chan error
}

func newBackendFake() *backendFake {
	return &backendFake{
		events: make(chan Event, 8),
		errors: make(chan error, 8),
	}
}

func (b *backendFake) Add(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.added = append(b.added, path)
	if b.addErr != nil {
		return b.addErr
	}
	return nil
}

func (b *backendFake) Remove(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.removed = append(b.removed, path)
	if b.removeErr != nil {
		return b.removeErr
	}
	return nil
}

func (b *backendFake) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return b.closeErr
	}
	b.closed = true
	close(b.events)
	close(b.errors)
	return b.closeErr
}

func (b *backendFake) Events() <-chan Event { return b.events }
func (b *backendFake) Errors() <-chan error { return b.errors }

func (b *backendFake) Added() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.added))
	copy(out, b.added)
	return out
}

func TestFallbackAddDegradesWhenNativeAddFails(t *testing.T) {
	native := newBackendFake()
	native.addErr = errors.New("native add failure")
	poll := newBackendFake()

	f := newFallbackWatcherWithBackends(native, poll, ModeNative)
	defer f.Close()

	if err := f.Add("./logs/app.log"); err != nil {
		t.Fatalf("add should succeed via polling fallback, got %v", err)
	}
	if f.Mode() != ModePolling {
		t.Fatalf("expected mode polling, got %s", f.Mode())
	}

	expected, _ := filepath.Abs(filepath.Clean("./logs/app.log"))
	added := poll.Added()
	if len(added) == 0 || added[len(added)-1] != expected {
		t.Fatalf("expected poll add for %s, got %+v", expected, added)
	}

	select {
	case err := <-f.Errors():
		if err == nil {
			t.Fatalf("expected fallback error signal")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected fallback error notification")
	}
}

func TestFallbackSwitchesOnNativeAsyncError(t *testing.T) {
	native := newBackendFake()
	poll := newBackendFake()
	f := newFallbackWatcherWithBackends(native, poll, ModeNative)
	defer f.Close()

	if err := f.Add("./logs/app.log"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	native.errors <- errors.New("native runtime failure")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f.Mode() == ModePolling {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if f.Mode() != ModePolling {
		t.Fatalf("expected polling mode after native runtime error")
	}
	if len(poll.Added()) == 0 {
		t.Fatalf("expected active paths to be registered with polling backend")
	}
}

func TestFallbackStartsPollingWhenNativeMissing(t *testing.T) {
	poll := newBackendFake()
	f := newFallbackWatcherWithBackends(nil, poll, ModePolling)
	defer f.Close()

	if f.Mode() != ModePolling {
		t.Fatalf("expected initial polling mode, got %s", f.Mode())
	}
	if err := f.Add("./logs/app.log"); err != nil {
		t.Fatalf("polling add failed: %v", err)
	}
	if added := poll.Added(); len(added) != 1 {
		t.Fatalf("expected one poll add, got %+v", added)
	}
}

func TestFallbackForwardsActiveModeEvents(t *testing.T) {
	native := newBackendFake()
	poll := newBackendFake()
	f := newFallbackWatcherWithBackends(native, poll, ModeNative)
	defer f.Close()

	native.events <- Event{Path: "a.log", Op: OpWrite}
	select {
	case ev := <-f.Events():
		if ev.Path != "a.log" || ev.Op != OpWrite {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected native event")
	}

	native.errors <- errors.New("native fail")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f.Mode() == ModePolling {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	poll.events <- Event{Path: "b.log", Op: OpCreate}
	select {
	case ev := <-f.Events():
		if ev.Path != "b.log" || ev.Op != OpCreate {
			t.Fatalf("unexpected poll event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected polling event")
	}
}
