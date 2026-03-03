package lastseen

import (
	"sync"
	"testing"
	"time"
)

func TestTrackerUpdateGetAndStaleClassification(t *testing.T) {
	tr := NewTracker()
	base := time.Date(2026, 3, 3, 12, 45, 0, 0, time.UTC)
	tr.now = func() time.Time { return base }

	tr.UpdateAt("source-1", base.Add(-2*time.Second))
	rec, ok := tr.Get("source-1", 5*time.Second)
	if !ok {
		t.Fatalf("expected record")
	}
	if rec.Status != StatusActive {
		t.Fatalf("expected active status, got %s", rec.Status)
	}

	tr.now = func() time.Time { return base.Add(10 * time.Second) }
	rec, ok = tr.Get("source-1", 5*time.Second)
	if !ok {
		t.Fatalf("expected record")
	}
	if rec.Status != StatusStale {
		t.Fatalf("expected stale status, got %s", rec.Status)
	}
}

func TestTrackerIgnoresOutOfOrderUpdates(t *testing.T) {
	tr := NewTracker()
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)

	tr.UpdateAt("source-1", now)
	tr.UpdateAt("source-1", now.Add(-time.Minute))

	rec, ok := tr.Get("source-1", 0)
	if !ok {
		t.Fatalf("expected record")
	}
	if !rec.LastSeenAt.Equal(now) {
		t.Fatalf("expected last seen to remain newest timestamp, got %s", rec.LastSeenAt)
	}
}

func TestTrackerSnapshotSortedAndPrune(t *testing.T) {
	tr := NewTracker()
	base := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	tr.now = func() time.Time { return base.Add(30 * time.Second) }

	tr.UpdateAt("source-b", base.Add(-20*time.Second))
	tr.UpdateAt("source-a", base.Add(20*time.Second))

	snapshot := tr.Snapshot(15 * time.Second)
	if len(snapshot) != 2 {
		t.Fatalf("expected 2 records, got %d", len(snapshot))
	}
	if snapshot[0].SourceID != "source-a" || snapshot[1].SourceID != "source-b" {
		t.Fatalf("snapshot not sorted by source id: %+v", snapshot)
	}
	if snapshot[1].Status != StatusStale {
		t.Fatalf("expected source-b to be stale: %+v", snapshot[1])
	}

	removed := tr.PruneOlderThan(15 * time.Second)
	if removed != 1 {
		t.Fatalf("expected 1 pruned record, got %d", removed)
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	tr := NewTracker()
	base := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	tr.now = func() time.Time { return base.Add(5 * time.Second) }

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "source-" + string(rune('a'+(i%4)))
			tr.UpdateAt(id, base.Add(time.Duration(i)*time.Millisecond))
			_, _ = tr.Get(id, time.Second)
		}(i)
	}
	wg.Wait()

	snapshot := tr.Snapshot(time.Second)
	if len(snapshot) == 0 {
		t.Fatalf("expected non-empty snapshot")
	}
}
