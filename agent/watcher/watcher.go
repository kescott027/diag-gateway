package watcher

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Op is a normalized file notification operation bitmask.
type Op uint8

const (
	OpCreate Op = 1 << iota
	OpWrite
	OpRemove
	OpRename
	OpChmod
)

// Event is a normalized file notification event.
type Event struct {
	Path string `json:"path"`
	Op   Op     `json:"op"`
}

type backend interface {
	Add(path string) error
	Remove(path string) error
	Close() error
	Events() <-chan Event
	Errors() <-chan error
}

// Watcher provides a platform-native file notification abstraction.
type Watcher struct {
	backend backend
	once    sync.Once
}

func NewNativeWatcher() (*Watcher, error) {
	b, err := newFSNotifyBackend()
	if err != nil {
		return nil, err
	}
	return &Watcher{backend: b}, nil
}

func newWatcherWithBackend(b backend) *Watcher {
	return &Watcher{backend: b}
}

func (w *Watcher) Add(path string) error {
	normalized, err := normalizePath(path)
	if err != nil {
		return err
	}
	return w.backend.Add(normalized)
}

func (w *Watcher) Remove(path string) error {
	normalized, err := normalizePath(path)
	if err != nil {
		return err
	}
	return w.backend.Remove(normalized)
}

func (w *Watcher) Events() <-chan Event {
	return w.backend.Events()
}

func (w *Watcher) Errors() <-chan error {
	return w.backend.Errors()
}

func (w *Watcher) Close() error {
	var closeErr error
	w.once.Do(func() {
		closeErr = w.backend.Close()
	})
	return closeErr
}

type fsnotifyBackend struct {
	watcher *fsnotify.Watcher
	events  chan Event
	errors  chan error
}

func newFSNotifyBackend() (*fsnotifyBackend, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create native watcher: %w", err)
	}

	b := &fsnotifyBackend{
		watcher: w,
		events:  make(chan Event, 128),
		errors:  make(chan error, 32),
	}
	go b.forward()
	return b, nil
}

func (b *fsnotifyBackend) Add(path string) error {
	return b.watcher.Add(path)
}

func (b *fsnotifyBackend) Remove(path string) error {
	return b.watcher.Remove(path)
}

func (b *fsnotifyBackend) Events() <-chan Event {
	return b.events
}

func (b *fsnotifyBackend) Errors() <-chan error {
	return b.errors
}

func (b *fsnotifyBackend) Close() error {
	return b.watcher.Close()
}

func (b *fsnotifyBackend) forward() {
	defer close(b.events)
	defer close(b.errors)

	for {
		select {
		case ev, ok := <-b.watcher.Events:
			if !ok {
				return
			}
			normalized := mapEvent(ev)
			if normalized.Op == 0 {
				continue
			}
			b.events <- normalized
		case err, ok := <-b.watcher.Errors:
			if !ok {
				return
			}
			b.errors <- err
		}
	}
}

func mapEvent(ev fsnotify.Event) Event {
	var op Op
	if ev.Has(fsnotify.Create) {
		op |= OpCreate
	}
	if ev.Has(fsnotify.Write) {
		op |= OpWrite
	}
	if ev.Has(fsnotify.Remove) {
		op |= OpRemove
	}
	if ev.Has(fsnotify.Rename) {
		op |= OpRename
	}
	if ev.Has(fsnotify.Chmod) {
		op |= OpChmod
	}
	return Event{
		Path: ev.Name,
		Op:   op,
	}
}

func normalizePath(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", fmt.Errorf("normalize path %q: %w", p, err)
	}
	return abs, nil
}
