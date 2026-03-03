package watcher

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type polledState struct {
	exists  bool
	size    int64
	modTime time.Time
}

type trackedPath struct {
	state        polledState
	lastActivity time.Time
	lastScanned  time.Time
}

type pollingBackend struct {
	policy AdaptivePollingPolicy
	now    func() time.Time

	mu    sync.Mutex
	paths map[string]trackedPath

	events chan Event
	errors chan error
	done   chan struct{}

	ticker    *time.Ticker
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func newPollingBackend(interval time.Duration) *pollingBackend {
	if interval <= 0 {
		interval = defaultPollInterval
	}
	policy := AdaptivePollingPolicy{
		HotInterval:  interval,
		WarmInterval: interval,
		ColdInterval: interval,
		WarmAfter:    24 * time.Hour,
		ColdAfter:    48 * time.Hour,
	}
	return newAdaptivePollingBackend(policy)
}

func newAdaptivePollingBackend(policy AdaptivePollingPolicy) *pollingBackend {
	normalized := policy.normalize()

	b := &pollingBackend{
		policy: normalized,
		now:    time.Now,
		paths:  make(map[string]trackedPath),
		events: make(chan Event, 128),
		errors: make(chan error, 32),
		done:   make(chan struct{}),
		ticker: time.NewTicker(normalized.MinInterval()),
	}
	b.wg.Add(1)
	go b.loop()
	return b
}

func (b *pollingBackend) Add(path string) error {
	state := currentState(path)
	now := b.now().UTC()
	lastActivity := time.Time{}
	if state.exists {
		lastActivity = now
	}

	b.mu.Lock()
	b.paths[path] = trackedPath{
		state:        state,
		lastActivity: lastActivity,
	}
	b.mu.Unlock()
	return nil
}

func (b *pollingBackend) Remove(path string) error {
	b.mu.Lock()
	delete(b.paths, path)
	b.mu.Unlock()
	return nil
}

func (b *pollingBackend) Events() <-chan Event {
	return b.events
}

func (b *pollingBackend) Errors() <-chan error {
	return b.errors
}

func (b *pollingBackend) Close() error {
	b.closeOnce.Do(func() {
		close(b.done)
		b.ticker.Stop()
		b.wg.Wait()
		close(b.events)
		close(b.errors)
	})
	return nil
}

func (b *pollingBackend) loop() {
	defer b.wg.Done()

	for {
		select {
		case <-b.done:
			return
		case <-b.ticker.C:
			b.scanOnce()
		}
	}
}

func (b *pollingBackend) scanOnce() {
	now := b.now().UTC()

	b.mu.Lock()
	defer b.mu.Unlock()

	for path, tracked := range b.paths {
		interval := b.policy.Interval(now, tracked.lastActivity)
		if !tracked.lastScanned.IsZero() && now.Sub(tracked.lastScanned) < interval {
			continue
		}
		tracked.lastScanned = now

		next, err := pollState(path)
		if err != nil {
			select {
			case b.errors <- err:
			case <-b.done:
				return
			}
			continue
		}

		activity := false
		if !tracked.state.exists && next.exists {
			b.emit(Event{Path: path, Op: OpCreate})
			activity = true
		}
		if tracked.state.exists && !next.exists {
			b.emit(Event{Path: path, Op: OpRemove})
			activity = true
		}
		if tracked.state.exists && next.exists && (tracked.state.size != next.size || !tracked.state.modTime.Equal(next.modTime)) {
			b.emit(Event{Path: path, Op: OpWrite})
			activity = true
		}
		if activity {
			tracked.lastActivity = now
		}

		tracked.state = next
		b.paths[path] = tracked
	}
}

func (b *pollingBackend) emit(ev Event) {
	select {
	case b.events <- ev:
	case <-b.done:
	}
}

func currentState(path string) polledState {
	state, err := pollState(path)
	if err != nil {
		return polledState{}
	}
	return state
}

func pollState(path string) (polledState, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return polledState{}, nil
		}
		return polledState{}, fmt.Errorf("poll stat %s: %w", path, err)
	}
	return polledState{
		exists:  true,
		size:    info.Size(),
		modTime: info.ModTime(),
	}, nil
}
