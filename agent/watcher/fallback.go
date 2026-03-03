package watcher

import (
	"fmt"
	"sync"
	"time"
)

// Mode is the active backend mode used by a fallback watcher.
type Mode string

const (
	ModeNative  Mode = "native"
	ModePolling Mode = "polling"
)

const defaultPollInterval = 500 * time.Millisecond

// FallbackWatcher runs native notifications first and degrades to polling on failure.
type FallbackWatcher struct {
	native backend
	poll   backend

	events chan Event
	errors chan error
	done   chan struct{}

	mu    sync.RWMutex
	mode  Mode
	paths map[string]struct{}

	wg        sync.WaitGroup
	closeOnce sync.Once
}

func NewFallbackWatcher(native *Watcher, pollInterval time.Duration) (*FallbackWatcher, error) {
	policy := DefaultAdaptivePollingPolicy()
	if pollInterval > 0 {
		policy.HotInterval = pollInterval
		if policy.WarmInterval < policy.HotInterval {
			policy.WarmInterval = policy.HotInterval
		}
		if policy.ColdInterval < policy.WarmInterval {
			policy.ColdInterval = policy.WarmInterval
		}
	}
	pb := newAdaptivePollingBackend(policy)
	var nb backend
	mode := ModePolling
	if native != nil {
		nb = native.backend
		mode = ModeNative
	}
	return newFallbackWatcherWithBackends(nb, pb, mode), nil
}

func newFallbackWatcherWithBackends(native, poll backend, mode Mode) *FallbackWatcher {
	f := &FallbackWatcher{
		native: native,
		poll:   poll,
		events: make(chan Event, 128),
		errors: make(chan error, 64),
		done:   make(chan struct{}),
		mode:   mode,
		paths:  make(map[string]struct{}),
	}

	if f.native != nil {
		f.wg.Add(1)
		go f.forward(f.native, ModeNative)
	}
	if f.poll != nil {
		f.wg.Add(1)
		go f.forward(f.poll, ModePolling)
	}
	return f
}

func (f *FallbackWatcher) Add(path string) error {
	normalized, err := normalizePath(path)
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.paths[normalized] = struct{}{}
	mode := f.mode
	f.mu.Unlock()

	if mode == ModeNative && f.native != nil {
		if err := f.native.Add(normalized); err == nil {
			return nil
		} else {
			f.switchToPolling(fmt.Errorf("native add failed for %s: %w", normalized, err))
		}
	}

	if err := f.poll.Add(normalized); err != nil {
		return fmt.Errorf("poll add failed for %s: %w", normalized, err)
	}
	return nil
}

func (f *FallbackWatcher) Remove(path string) error {
	normalized, err := normalizePath(path)
	if err != nil {
		return err
	}

	f.mu.Lock()
	delete(f.paths, normalized)
	mode := f.mode
	f.mu.Unlock()

	if mode == ModeNative && f.native != nil {
		if err := f.native.Remove(normalized); err != nil {
			f.switchToPolling(fmt.Errorf("native remove failed for %s: %w", normalized, err))
			if err := f.poll.Remove(normalized); err != nil {
				return fmt.Errorf("poll remove failed for %s: %w", normalized, err)
			}
		}
		return nil
	}

	if err := f.poll.Remove(normalized); err != nil {
		return fmt.Errorf("poll remove failed for %s: %w", normalized, err)
	}
	return nil
}

func (f *FallbackWatcher) Events() <-chan Event {
	return f.events
}

func (f *FallbackWatcher) Errors() <-chan error {
	return f.errors
}

func (f *FallbackWatcher) Mode() Mode {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.mode
}

func (f *FallbackWatcher) Close() error {
	var closeErr error
	f.closeOnce.Do(func() {
		close(f.done)
		if f.native != nil {
			if err := f.native.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		if f.poll != nil {
			if err := f.poll.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		f.wg.Wait()
		close(f.events)
		close(f.errors)
	})
	return closeErr
}

func (f *FallbackWatcher) forward(b backend, mode Mode) {
	defer f.wg.Done()

	for {
		select {
		case <-f.done:
			return
		case ev, ok := <-b.Events():
			if !ok {
				return
			}
			if f.Mode() != mode {
				continue
			}
			select {
			case f.events <- ev:
			case <-f.done:
				return
			}
		case err, ok := <-b.Errors():
			if !ok {
				return
			}
			if mode == ModeNative {
				if f.Mode() == ModeNative {
					f.switchToPolling(fmt.Errorf("native watcher error: %w", err))
				}
				continue
			}
			if f.Mode() != ModePolling {
				continue
			}
			select {
			case f.errors <- fmt.Errorf("polling watcher error: %w", err):
			case <-f.done:
				return
			}
		}
	}
}

func (f *FallbackWatcher) switchToPolling(cause error) {
	f.mu.Lock()
	if f.mode == ModePolling {
		f.mu.Unlock()
		return
	}
	f.mode = ModePolling
	paths := make([]string, 0, len(f.paths))
	for path := range f.paths {
		paths = append(paths, path)
	}
	f.mu.Unlock()

	select {
	case f.errors <- fmt.Errorf("native watcher degraded to polling fallback: %w", cause):
	case <-f.done:
		return
	}

	for _, path := range paths {
		if err := f.poll.Add(path); err != nil {
			select {
			case f.errors <- fmt.Errorf("poll add failed during failover for %s: %w", path, err):
			case <-f.done:
				return
			}
		}
	}

	if f.native != nil {
		_ = f.native.Close()
	}
}
