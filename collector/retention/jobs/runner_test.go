package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/metadata/store"
	"github.com/kescott027/diag-gateway/collector/retention"
)

type fakeLister struct {
	sourceIDs []string
	err       error
}

func (f fakeLister) ListSourceIDs(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]string, len(f.sourceIDs))
	copy(out, f.sourceIDs)
	return out, nil
}

type fakePruner struct {
	mu      sync.Mutex
	calls   []string
	results map[string]retention.Result
	errors  map[string]error
	block   chan struct{}
	started chan string
}

func (f *fakePruner) PruneSource(sourceID string) (retention.Result, error) {
	if f.started != nil {
		f.started <- sourceID
	}
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	f.calls = append(f.calls, sourceID)
	f.mu.Unlock()
	if err := f.errors[sourceID]; err != nil {
		return retention.Result{}, err
	}
	if res, ok := f.results[sourceID]; ok {
		return res, nil
	}
	return retention.Result{SourceID: sourceID}, nil
}

func (f *fakePruner) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

type fakeCompactor struct {
	mu      sync.Mutex
	calls   []string
	results map[string]CompactionResult
	errors  map[string]error
}

func (f *fakeCompactor) CompactSource(_ context.Context, sourceID string) (CompactionResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, sourceID)
	f.mu.Unlock()
	if err := f.errors[sourceID]; err != nil {
		return CompactionResult{}, err
	}
	if res, ok := f.results[sourceID]; ok {
		return res, nil
	}
	return CompactionResult{SourceID: sourceID, Note: "ok"}, nil
}

func (f *fakeCompactor) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestRunnerTickScheduleAndBounds(t *testing.T) {
	pruner := &fakePruner{
		results: map[string]retention.Result{
			"source-a": {SourceID: "source-a", StreamsDeleted: 1},
			"source-b": {SourceID: "source-b", ArtifactsDeleted: 2},
		},
		errors: map[string]error{"source-c": errors.New("prune failed")},
	}
	compactor := &fakeCompactor{
		results: map[string]CompactionResult{
			"source-a": {SourceID: "source-a", Compacted: true, BytesBefore: 100, BytesAfter: 40},
			"source-b": {SourceID: "source-b", Compacted: false, Note: "already compact"},
		},
	}
	runner, err := NewRunner(Config{
		RetentionInterval:  time.Minute,
		CompactionInterval: 2 * time.Minute,
		MaxSourcesPerRun:   2,
	}, fakeLister{sourceIDs: []string{"source-c", "source-a", "source-b", "source-a", ""}}, pruner, compactor)
	if err != nil {
		t.Fatalf("new runner failed: %v", err)
	}

	now := time.Date(2026, 3, 3, 14, 30, 0, 0, time.UTC)
	runner.now = func() time.Time { return now }

	tick1, err := runner.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick1 failed: %v", err)
	}
	if !tick1.Retention.Due || !tick1.Compaction.Due {
		t.Fatalf("expected both jobs due on first tick: %+v", tick1)
	}
	if tick1.Retention.SourcesTotal != 3 || tick1.Retention.SourcesRun != 2 || tick1.Retention.SourcesDeferred != 1 {
		t.Fatalf("unexpected retention source bounds: %+v", tick1.Retention)
	}
	if len(tick1.Retention.Results) != 2 || len(tick1.Retention.Failures) != 0 {
		t.Fatalf("unexpected retention execution summary: %+v", tick1.Retention)
	}
	if tick1.Compaction.SourcesTotal != 3 || tick1.Compaction.SourcesRun != 2 || tick1.Compaction.SourcesDeferred != 1 {
		t.Fatalf("unexpected compaction source bounds: %+v", tick1.Compaction)
	}
	if calls := pruner.Calls(); len(calls) != 2 || calls[0] != "source-a" || calls[1] != "source-b" {
		t.Fatalf("unexpected pruner call order: %+v", calls)
	}
	if calls := compactor.Calls(); len(calls) != 2 || calls[0] != "source-a" || calls[1] != "source-b" {
		t.Fatalf("unexpected compactor call order: %+v", calls)
	}

	now = now.Add(30 * time.Second)
	tick2, err := runner.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick2 failed: %v", err)
	}
	if tick2.Retention.Due || tick2.Compaction.Due {
		t.Fatalf("expected no due jobs before interval gates: %+v", tick2)
	}

	now = now.Add(35 * time.Second)
	tick3, err := runner.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick3 failed: %v", err)
	}
	if !tick3.Retention.Due || tick3.Compaction.Due {
		t.Fatalf("expected retention only due after 65s: %+v", tick3)
	}
}

func TestRunnerRetentionOverlapSkipsSecondTick(t *testing.T) {
	started := make(chan string, 1)
	block := make(chan struct{})
	pruner := &fakePruner{
		results: map[string]retention.Result{"source-a": {SourceID: "source-a"}},
		errors:  map[string]error{},
		block:   block,
		started: started,
	}
	runner, err := NewRunner(Config{
		RetentionInterval:  time.Minute,
		CompactionInterval: time.Hour,
		MaxSourcesPerRun:   10,
	}, fakeLister{sourceIDs: []string{"source-a"}}, pruner, &fakeCompactor{results: map[string]CompactionResult{}, errors: map[string]error{}})
	if err != nil {
		t.Fatalf("new runner failed: %v", err)
	}
	now := time.Date(2026, 3, 3, 14, 40, 0, 0, time.UTC)
	runner.now = func() time.Time { return now }

	var wg sync.WaitGroup
	wg.Add(1)
	var firstErr error
	go func() {
		defer wg.Done()
		_, firstErr = runner.Tick(context.Background())
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("first tick did not start prune in time")
	}

	secondTick, err := runner.Tick(context.Background())
	if err != nil {
		t.Fatalf("second tick failed: %v", err)
	}
	if !secondTick.Retention.Due || !secondTick.Retention.SkippedOverlap {
		t.Fatalf("expected second retention tick to skip overlap: %+v", secondTick.Retention)
	}

	close(block)
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("first tick error: %v", firstErr)
	}
}

func TestMetadataSourceLister(t *testing.T) {
	mem := store.NewMemoryStore()
	t.Cleanup(func() {
		_ = mem.Close()
	})
	ctx := context.Background()
	if err := mem.UpsertSource(ctx, store.SourceRecord{SourceID: "source-b", Status: "active", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("upsert source-b failed: %v", err)
	}
	if err := mem.UpsertSource(ctx, store.SourceRecord{SourceID: "source-a", Status: "active", UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("upsert source-a failed: %v", err)
	}

	lister := MetadataSourceLister{Store: mem}
	ids, err := lister.ListSourceIDs(ctx)
	if err != nil {
		t.Fatalf("list source ids failed: %v", err)
	}
	if len(ids) != 2 || ids[0] != "source-a" || ids[1] != "source-b" {
		t.Fatalf("unexpected source ids: %+v", ids)
	}
}
