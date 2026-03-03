package watcher

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fsnotify/fsnotify"
)

type fakeBackend struct {
	added    []string
	removed  []string
	closeCnt int
	events   chan Event
	errors   chan error
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		events: make(chan Event, 4),
		errors: make(chan error, 4),
	}
}

func (f *fakeBackend) Add(path string) error {
	f.added = append(f.added, path)
	return nil
}

func (f *fakeBackend) Remove(path string) error {
	f.removed = append(f.removed, path)
	return nil
}

func (f *fakeBackend) Close() error {
	f.closeCnt++
	return nil
}

func (f *fakeBackend) Events() <-chan Event {
	return f.events
}

func (f *fakeBackend) Errors() <-chan error {
	return f.errors
}

func TestWatcherAddRemoveNormalizesPath(t *testing.T) {
	b := newFakeBackend()
	w := newWatcherWithBackend(b)

	if err := w.Add(" ./logs/app.log "); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if err := w.Remove(" ./logs/app.log "); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	expected, err := filepath.Abs(filepath.Clean("./logs/app.log"))
	if err != nil {
		t.Fatalf("abs failed: %v", err)
	}
	if len(b.added) != 1 || b.added[0] != expected {
		t.Fatalf("unexpected added paths: %+v expected %s", b.added, expected)
	}
	if len(b.removed) != 1 || b.removed[0] != expected {
		t.Fatalf("unexpected removed paths: %+v expected %s", b.removed, expected)
	}
}

func TestWatcherRejectsEmptyPath(t *testing.T) {
	b := newFakeBackend()
	w := newWatcherWithBackend(b)
	if err := w.Add("   "); err == nil {
		t.Fatalf("expected empty path rejection")
	}
	if err := w.Remove(""); err == nil {
		t.Fatalf("expected empty path rejection")
	}
}

func TestWatcherCloseIsIdempotent(t *testing.T) {
	b := newFakeBackend()
	w := newWatcherWithBackend(b)

	if err := w.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if b.closeCnt != 1 {
		t.Fatalf("expected one backend close call, got %d", b.closeCnt)
	}
}

func TestWatcherEventAndErrorChannelsPassthrough(t *testing.T) {
	b := newFakeBackend()
	w := newWatcherWithBackend(b)

	sample := Event{Path: "x", Op: OpWrite}
	b.events <- sample
	got := <-w.Events()
	if got != sample {
		t.Fatalf("unexpected event: %+v", got)
	}

	sampleErr := errors.New("boom")
	b.errors <- sampleErr
	if gotErr := <-w.Errors(); !errors.Is(gotErr, sampleErr) {
		t.Fatalf("unexpected error: %v", gotErr)
	}
}

func TestMapEventCoversAllOps(t *testing.T) {
	mapped := mapEvent(fsnotify.Event{
		Name: "sample.log",
		Op:   fsnotify.Create | fsnotify.Write | fsnotify.Remove | fsnotify.Rename | fsnotify.Chmod,
	})
	if mapped.Path != "sample.log" {
		t.Fatalf("unexpected mapped path: %s", mapped.Path)
	}

	expected := OpCreate | OpWrite | OpRemove | OpRename | OpChmod
	if mapped.Op != expected {
		t.Fatalf("unexpected mapped ops: got=%v expected=%v", mapped.Op, expected)
	}
}

func TestNewNativeWatcherSmoke(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("fsnotify watcher unsupported")
	}
	w, err := NewNativeWatcher()
	if err != nil {
		t.Fatalf("new native watcher failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close native watcher failed: %v", err)
	}
}
