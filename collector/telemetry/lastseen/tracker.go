package lastseen

import (
	"sort"
	"sync"
	"time"
)

type Status string

const (
	StatusActive Status = "active"
	StatusStale  Status = "stale"
)

// Record captures source liveness at query time.
type Record struct {
	SourceID   string    `json:"source_id"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Status     Status    `json:"status"`
	AgeSeconds float64   `json:"age_seconds"`
}

// Tracker stores last-seen timestamps per source.
type Tracker struct {
	now func() time.Time

	mu   sync.RWMutex
	seen map[string]time.Time
}

func NewTracker() *Tracker {
	return &Tracker{
		now:  time.Now,
		seen: make(map[string]time.Time),
	}
}

func (t *Tracker) Update(sourceID string) {
	t.UpdateAt(sourceID, t.now())
}

func (t *Tracker) UpdateAt(sourceID string, seenAt time.Time) {
	if sourceID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	current, ok := t.seen[sourceID]
	if ok && seenAt.Before(current) {
		return
	}
	t.seen[sourceID] = seenAt.UTC()
}

func (t *Tracker) Get(sourceID string, staleAfter time.Duration) (Record, bool) {
	t.mu.RLock()
	seenAt, ok := t.seen[sourceID]
	t.mu.RUnlock()
	if !ok {
		return Record{}, false
	}

	now := t.now().UTC()
	age := now.Sub(seenAt)
	return Record{
		SourceID:   sourceID,
		LastSeenAt: seenAt,
		Status:     classify(age, staleAfter),
		AgeSeconds: age.Seconds(),
	}, true
}

func (t *Tracker) Snapshot(staleAfter time.Duration) []Record {
	t.mu.RLock()
	clone := make(map[string]time.Time, len(t.seen))
	for k, v := range t.seen {
		clone[k] = v
	}
	t.mu.RUnlock()

	now := t.now().UTC()
	out := make([]Record, 0, len(clone))
	for sourceID, seenAt := range clone {
		age := now.Sub(seenAt)
		out = append(out, Record{
			SourceID:   sourceID,
			LastSeenAt: seenAt,
			Status:     classify(age, staleAfter),
			AgeSeconds: age.Seconds(),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].SourceID < out[j].SourceID })
	return out
}

func (t *Tracker) PruneOlderThan(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}

	now := t.now().UTC()
	removed := 0

	t.mu.Lock()
	defer t.mu.Unlock()
	for sourceID, seenAt := range t.seen {
		if now.Sub(seenAt) > maxAge {
			delete(t.seen, sourceID)
			removed++
		}
	}
	return removed
}

func classify(age, staleAfter time.Duration) Status {
	if staleAfter > 0 && age > staleAfter {
		return StatusStale
	}
	return StatusActive
}

