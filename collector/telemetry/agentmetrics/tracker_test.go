package agentmetrics

import (
	"sync"
	"testing"
	"time"
)

func TestTrackerGetTotalsAndRates(t *testing.T) {
	tr := NewTracker(10 * time.Second)
	base := time.Date(2026, 3, 3, 13, 2, 0, 0, time.UTC)
	tr.now = func() time.Time { return base }

	tr.AddBytes("source-1", 100, base.Add(-5*time.Second))
	tr.AddErrors("source-1", 2, base.Add(-2*time.Second))
	tr.SetQueueDepth("source-1", 7, base.Add(-1*time.Second))

	rec, ok := tr.Get("source-1")
	if !ok {
		t.Fatalf("expected source metrics record")
	}
	if rec.TotalBytes != 100 || rec.TotalErrors != 2 || rec.QueueDepth != 7 {
		t.Fatalf("unexpected totals: %+v", rec)
	}
	if rec.ThroughputBytesPerSecond != 10 {
		t.Fatalf("unexpected throughput: %f", rec.ThroughputBytesPerSecond)
	}
	if rec.ErrorRatePerSecond != 0.2 {
		t.Fatalf("unexpected error rate: %f", rec.ErrorRatePerSecond)
	}
}

func TestTrackerWindowExcludesStaleEvents(t *testing.T) {
	tr := NewTracker(10 * time.Second)
	base := time.Date(2026, 3, 3, 13, 3, 0, 0, time.UTC)
	tr.now = func() time.Time { return base }

	tr.AddBytes("source-1", 100, base.Add(-20*time.Second))
	tr.AddBytes("source-1", 30, base.Add(-2*time.Second))
	tr.AddErrors("source-1", 10, base.Add(-15*time.Second))
	tr.AddErrors("source-1", 1, base.Add(-1*time.Second))

	rec, ok := tr.Get("source-1")
	if !ok {
		t.Fatalf("expected source metrics record")
	}
	if rec.TotalBytes != 130 || rec.TotalErrors != 11 {
		t.Fatalf("unexpected totals: %+v", rec)
	}
	if rec.ThroughputBytesPerSecond != 3 {
		t.Fatalf("unexpected throughput: %f", rec.ThroughputBytesPerSecond)
	}
	if rec.ErrorRatePerSecond != 0.1 {
		t.Fatalf("unexpected error rate: %f", rec.ErrorRatePerSecond)
	}
}

func TestTrackerMonotonicTimestampPolicy(t *testing.T) {
	tr := NewTracker(10 * time.Second)
	base := time.Date(2026, 3, 3, 13, 4, 0, 0, time.UTC)
	tr.now = func() time.Time { return base }

	t1 := base.Add(-2 * time.Second)
	t0 := base.Add(-5 * time.Second)

	tr.AddBytes("source-1", 50, t1)
	tr.AddBytes("source-1", 50, t0)
	tr.SetQueueDepth("source-1", 1, t0)

	rec, ok := tr.Get("source-1")
	if !ok {
		t.Fatalf("expected record")
	}
	if !rec.UpdatedAt.Equal(t1) {
		t.Fatalf("expected monotonic updated time %s, got %s", t1, rec.UpdatedAt)
	}
	if rec.TotalBytes != 100 {
		t.Fatalf("expected total bytes to include both updates, got %d", rec.TotalBytes)
	}
}

func TestTrackerSnapshotSortedAndPruneInactive(t *testing.T) {
	tr := NewTracker(10 * time.Second)
	base := time.Date(2026, 3, 3, 13, 5, 0, 0, time.UTC)
	tr.now = func() time.Time { return base }

	tr.SetQueueDepth("source-b", 3, base.Add(-30*time.Second))
	tr.SetQueueDepth("source-a", 5, base.Add(-2*time.Second))

	snapshot := tr.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("expected 2 records, got %d", len(snapshot))
	}
	if snapshot[0].SourceID != "source-a" || snapshot[1].SourceID != "source-b" {
		t.Fatalf("expected sorted records, got %+v", snapshot)
	}

	removed := tr.PruneInactive(15 * time.Second)
	if removed != 1 {
		t.Fatalf("expected one pruned source, got %d", removed)
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	tr := NewTracker(15 * time.Second)
	base := time.Date(2026, 3, 3, 13, 6, 0, 0, time.UTC)
	tr.now = func() time.Time { return base }

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sourceID := "source-" + string(rune('a'+(i%8)))
			tr.AddBytes(sourceID, int64(i+1), base.Add(-time.Duration(i%10)*time.Second))
			tr.AddErrors(sourceID, int64(i%3), base.Add(-time.Duration(i%10)*time.Second))
			tr.SetQueueDepth(sourceID, int64(i%5), base.Add(-time.Duration(i%10)*time.Second))
			_, _ = tr.Get(sourceID)
		}(i)
	}
	wg.Wait()

	snapshot := tr.Snapshot()
	if len(snapshot) == 0 {
		t.Fatalf("expected non-empty snapshot")
	}
}

