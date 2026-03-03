package agentmetrics

import (
	"sort"
	"sync"
	"time"
)

const defaultRateWindow = time.Minute

type event struct {
	at    time.Time
	value int64
}

type sourceState struct {
	totalBytes  int64
	totalErrors int64
	queueDepth  int64
	updatedAt   time.Time
	byteEvents  []event
	errorEvents []event
}

// Record is the per-source metrics snapshot exposed to control-plane consumers.
type Record struct {
	SourceID                 string    `json:"source_id"`
	TotalBytes               int64     `json:"total_bytes"`
	TotalErrors              int64     `json:"total_errors"`
	QueueDepth               int64     `json:"queue_depth"`
	ThroughputBytesPerSecond float64   `json:"throughput_bytes_per_second"`
	ErrorRatePerSecond       float64   `json:"error_rate_per_second"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// Tracker stores per-agent telemetry with deterministic fixed-window rate calculations.
type Tracker struct {
	window time.Duration
	now    func() time.Time

	mu     sync.RWMutex
	states map[string]*sourceState
}

func NewTracker(window time.Duration) *Tracker {
	if window <= 0 {
		window = defaultRateWindow
	}
	return &Tracker{
		window: window,
		now:    time.Now,
		states: make(map[string]*sourceState),
	}
}

func (t *Tracker) AddBytes(sourceID string, bytes int64, at time.Time) {
	if sourceID == "" || bytes <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now().UTC()
	ts := normalizeTimestamp(at, now)
	st := t.getOrCreate(sourceID)

	st.totalBytes += bytes
	st.byteEvents = append(st.byteEvents, event{at: ts, value: bytes})
	if ts.After(st.updatedAt) {
		st.updatedAt = ts
	}
	t.pruneLocked(st, now)
}

func (t *Tracker) AddErrors(sourceID string, errors int64, at time.Time) {
	if sourceID == "" || errors <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now().UTC()
	ts := normalizeTimestamp(at, now)
	st := t.getOrCreate(sourceID)

	st.totalErrors += errors
	st.errorEvents = append(st.errorEvents, event{at: ts, value: errors})
	if ts.After(st.updatedAt) {
		st.updatedAt = ts
	}
	t.pruneLocked(st, now)
}

func (t *Tracker) SetQueueDepth(sourceID string, depth int64, at time.Time) {
	if sourceID == "" {
		return
	}
	if depth < 0 {
		depth = 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now().UTC()
	ts := normalizeTimestamp(at, now)
	st := t.getOrCreate(sourceID)
	if ts.Before(st.updatedAt) {
		return
	}

	st.queueDepth = depth
	st.updatedAt = ts
	t.pruneLocked(st, now)
}

func (t *Tracker) Get(sourceID string) (Record, bool) {
	t.mu.RLock()
	st, ok := t.states[sourceID]
	if !ok {
		t.mu.RUnlock()
		return Record{}, false
	}

	now := t.now().UTC()
	rec := t.recordLocked(sourceID, st, now)
	t.mu.RUnlock()
	return rec, true
}

func (t *Tracker) Snapshot() []Record {
	t.mu.RLock()
	now := t.now().UTC()
	out := make([]Record, 0, len(t.states))
	for sourceID, st := range t.states {
		out = append(out, t.recordLocked(sourceID, st, now))
	}
	t.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].SourceID < out[j].SourceID
	})
	return out
}

func (t *Tracker) PruneInactive(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}
	now := t.now().UTC()

	removed := 0
	t.mu.Lock()
	defer t.mu.Unlock()
	for sourceID, st := range t.states {
		if now.Sub(st.updatedAt) > maxAge {
			delete(t.states, sourceID)
			removed++
		}
	}
	return removed
}

func (t *Tracker) getOrCreate(sourceID string) *sourceState {
	st, ok := t.states[sourceID]
	if ok {
		return st
	}
	st = &sourceState{}
	t.states[sourceID] = st
	return st
}

func (t *Tracker) pruneLocked(st *sourceState, anchor time.Time) {
	cutoff := anchor.Add(-t.window)
	st.byteEvents = pruneEvents(st.byteEvents, cutoff)
	st.errorEvents = pruneEvents(st.errorEvents, cutoff)
}

func (t *Tracker) recordLocked(sourceID string, st *sourceState, now time.Time) Record {
	cutoff := now.Add(-t.window)
	windowBytes := sumWithinWindow(st.byteEvents, cutoff)
	windowErrors := sumWithinWindow(st.errorEvents, cutoff)
	windowSeconds := t.window.Seconds()

	return Record{
		SourceID:                 sourceID,
		TotalBytes:               st.totalBytes,
		TotalErrors:              st.totalErrors,
		QueueDepth:               st.queueDepth,
		ThroughputBytesPerSecond: float64(windowBytes) / windowSeconds,
		ErrorRatePerSecond:       float64(windowErrors) / windowSeconds,
		UpdatedAt:                st.updatedAt,
	}
}

func pruneEvents(events []event, cutoff time.Time) []event {
	if len(events) == 0 {
		return events
	}

	start := 0
	for start < len(events) && events[start].at.Before(cutoff) {
		start++
	}
	if start == 0 {
		return events
	}
	if start >= len(events) {
		return nil
	}

	out := make([]event, len(events)-start)
	copy(out, events[start:])
	return out
}

func sumWithinWindow(events []event, cutoff time.Time) int64 {
	var total int64
	for _, e := range events {
		if e.at.Before(cutoff) {
			continue
		}
		total += e.value
	}
	return total
}

func normalizeTimestamp(at, fallback time.Time) time.Time {
	if at.IsZero() {
		return fallback
	}
	return at.UTC()
}
