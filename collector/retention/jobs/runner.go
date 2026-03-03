package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kescott027/diag-gateway/collector/metadata/store"
	"github.com/kescott027/diag-gateway/collector/retention"
)

const (
	defaultRetentionInterval  = 5 * time.Minute
	defaultCompactionInterval = 10 * time.Minute
)

// SourceLister returns source IDs eligible for background jobs.
type SourceLister interface {
	ListSourceIDs(ctx context.Context) ([]string, error)
}

// RetentionPruner applies per-source retention.
type RetentionPruner interface {
	PruneSource(sourceID string) (retention.Result, error)
}

// Compactor applies per-source compaction hooks.
type Compactor interface {
	CompactSource(ctx context.Context, sourceID string) (CompactionResult, error)
}

// Config defines job runner scheduling and workload bounds.
type Config struct {
	RetentionInterval  time.Duration `json:"retention_interval"`
	CompactionInterval time.Duration `json:"compaction_interval"`
	MaxSourcesPerRun   int           `json:"max_sources_per_run"`
}

// SourceError is a source-scoped failure record.
type SourceError struct {
	SourceID string `json:"source_id"`
	Error    string `json:"error"`
}

// RetentionRunResult summarizes one retention run decision/execution.
type RetentionRunResult struct {
	Due             bool               `json:"due"`
	SkippedOverlap  bool               `json:"skipped_overlap"`
	StartedAt       time.Time          `json:"started_at,omitempty"`
	CompletedAt     time.Time          `json:"completed_at,omitempty"`
	SourcesTotal    int                `json:"sources_total"`
	SourcesRun      int                `json:"sources_run"`
	SourcesDeferred int                `json:"sources_deferred"`
	Results         []retention.Result `json:"results,omitempty"`
	Failures        []SourceError      `json:"failures,omitempty"`
}

// CompactionResult summarizes one source compaction hook execution.
type CompactionResult struct {
	SourceID    string `json:"source_id"`
	Compacted   bool   `json:"compacted"`
	BytesBefore int64  `json:"bytes_before"`
	BytesAfter  int64  `json:"bytes_after"`
	Note        string `json:"note,omitempty"`
}

// CompactionRunResult summarizes one compaction run decision/execution.
type CompactionRunResult struct {
	Due             bool               `json:"due"`
	SkippedOverlap  bool               `json:"skipped_overlap"`
	StartedAt       time.Time          `json:"started_at,omitempty"`
	CompletedAt     time.Time          `json:"completed_at,omitempty"`
	SourcesTotal    int                `json:"sources_total"`
	SourcesRun      int                `json:"sources_run"`
	SourcesDeferred int                `json:"sources_deferred"`
	Results         []CompactionResult `json:"results,omitempty"`
	Failures        []SourceError      `json:"failures,omitempty"`
}

// TickResult summarizes both retention and compaction decisions in one scheduler tick.
type TickResult struct {
	At         time.Time           `json:"at"`
	Retention  RetentionRunResult  `json:"retention"`
	Compaction CompactionRunResult `json:"compaction"`
}

// Runner executes retention and compaction jobs with bounded source throughput.
type Runner struct {
	cfg       Config
	lister    SourceLister
	pruner    RetentionPruner
	compactor Compactor
	now       func() time.Time

	mu                sync.Mutex
	runningRetention  bool
	runningCompaction bool
	lastRetention     time.Time
	lastCompaction    time.Time
}

// NewRunner constructs a bounded, deterministic job runner.
func NewRunner(cfg Config, lister SourceLister, pruner RetentionPruner, compactor Compactor) (*Runner, error) {
	if lister == nil {
		return nil, fmt.Errorf("source lister is required")
	}
	if pruner == nil {
		return nil, fmt.Errorf("retention pruner is required")
	}
	if compactor == nil {
		compactor = NoopCompactor{}
	}
	normalized := cfg
	if normalized.RetentionInterval <= 0 {
		normalized.RetentionInterval = defaultRetentionInterval
	}
	if normalized.CompactionInterval <= 0 {
		normalized.CompactionInterval = defaultCompactionInterval
	}
	if normalized.MaxSourcesPerRun <= 0 {
		normalized.MaxSourcesPerRun = 1000
	}
	return &Runner{
		cfg:       normalized,
		lister:    lister,
		pruner:    pruner,
		compactor: compactor,
		now:       time.Now,
	}, nil
}

// Tick performs one scheduling decision and executes due jobs synchronously.
func (r *Runner) Tick(ctx context.Context) (TickResult, error) {
	now := r.now().UTC()
	out := TickResult{At: now}

	retentionResult, retentionErr := r.tickRetention(ctx, now)
	out.Retention = retentionResult

	compactionResult, compactionErr := r.tickCompaction(ctx, now)
	out.Compaction = compactionResult

	if retentionErr != nil && compactionErr != nil {
		return out, fmt.Errorf("retention error: %v; compaction error: %w", retentionErr, compactionErr)
	}
	if retentionErr != nil {
		return out, retentionErr
	}
	if compactionErr != nil {
		return out, compactionErr
	}
	return out, nil
}

func (r *Runner) tickRetention(ctx context.Context, now time.Time) (RetentionRunResult, error) {
	r.mu.Lock()
	due := shouldRun(now, r.lastRetention, r.cfg.RetentionInterval)
	out := RetentionRunResult{Due: due}
	if !due {
		r.mu.Unlock()
		return out, nil
	}
	if r.runningRetention {
		out.SkippedOverlap = true
		r.mu.Unlock()
		return out, nil
	}
	r.runningRetention = true
	r.mu.Unlock()

	execOut, execErr := r.runRetention(ctx)

	r.mu.Lock()
	r.runningRetention = false
	r.lastRetention = now
	r.mu.Unlock()
	return execOut, execErr
}

func (r *Runner) tickCompaction(ctx context.Context, now time.Time) (CompactionRunResult, error) {
	r.mu.Lock()
	due := shouldRun(now, r.lastCompaction, r.cfg.CompactionInterval)
	out := CompactionRunResult{Due: due}
	if !due {
		r.mu.Unlock()
		return out, nil
	}
	if r.runningCompaction {
		out.SkippedOverlap = true
		r.mu.Unlock()
		return out, nil
	}
	r.runningCompaction = true
	r.mu.Unlock()

	execOut, execErr := r.runCompaction(ctx)

	r.mu.Lock()
	r.runningCompaction = false
	r.lastCompaction = now
	r.mu.Unlock()
	return execOut, execErr
}

func (r *Runner) runRetention(ctx context.Context) (RetentionRunResult, error) {
	start := r.now().UTC()
	sourceIDs, err := r.lister.ListSourceIDs(ctx)
	if err != nil {
		return RetentionRunResult{
			Due:         true,
			StartedAt:   start,
			CompletedAt: r.now().UTC(),
		}, fmt.Errorf("list retention sources: %w", err)
	}

	normalized := normalizeSourceIDs(sourceIDs)
	selected, deferred := applySourceLimit(normalized, r.cfg.MaxSourcesPerRun)
	out := RetentionRunResult{
		Due:             true,
		StartedAt:       start,
		SourcesTotal:    len(normalized),
		SourcesDeferred: deferred,
		Results:         make([]retention.Result, 0, len(selected)),
		Failures:        make([]SourceError, 0),
	}

	for _, sourceID := range selected {
		res, pruneErr := r.pruner.PruneSource(sourceID)
		if pruneErr != nil {
			out.Failures = append(out.Failures, SourceError{SourceID: sourceID, Error: pruneErr.Error()})
			continue
		}
		out.Results = append(out.Results, res)
	}
	out.SourcesRun = len(selected)
	out.CompletedAt = r.now().UTC()
	return out, nil
}

func (r *Runner) runCompaction(ctx context.Context) (CompactionRunResult, error) {
	start := r.now().UTC()
	sourceIDs, err := r.lister.ListSourceIDs(ctx)
	if err != nil {
		return CompactionRunResult{
			Due:         true,
			StartedAt:   start,
			CompletedAt: r.now().UTC(),
		}, fmt.Errorf("list compaction sources: %w", err)
	}

	normalized := normalizeSourceIDs(sourceIDs)
	selected, deferred := applySourceLimit(normalized, r.cfg.MaxSourcesPerRun)
	out := CompactionRunResult{
		Due:             true,
		StartedAt:       start,
		SourcesTotal:    len(normalized),
		SourcesDeferred: deferred,
		Results:         make([]CompactionResult, 0, len(selected)),
		Failures:        make([]SourceError, 0),
	}

	for _, sourceID := range selected {
		res, compactErr := r.compactor.CompactSource(ctx, sourceID)
		if compactErr != nil {
			out.Failures = append(out.Failures, SourceError{SourceID: sourceID, Error: compactErr.Error()})
			continue
		}
		out.Results = append(out.Results, res)
	}
	out.SourcesRun = len(selected)
	out.CompletedAt = r.now().UTC()
	return out, nil
}

func shouldRun(now, lastRun time.Time, interval time.Duration) bool {
	if lastRun.IsZero() {
		return true
	}
	return now.Sub(lastRun) >= interval
}

func normalizeSourceIDs(ids []string) []string {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func applySourceLimit(ids []string, maxSources int) ([]string, int) {
	if maxSources <= 0 || len(ids) <= maxSources {
		return ids, 0
	}
	selected := make([]string, maxSources)
	copy(selected, ids[:maxSources])
	return selected, len(ids) - maxSources
}

// MetadataSourceLister adapts metadata store source records into scheduler source IDs.
type MetadataSourceLister struct {
	Store store.Store
}

func (l MetadataSourceLister) ListSourceIDs(ctx context.Context) ([]string, error) {
	if l.Store == nil {
		return nil, fmt.Errorf("metadata store is required")
	}
	rows, err := l.Store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.SourceID)
	}
	return normalizeSourceIDs(out), nil
}

// NoopCompactor provides a deterministic no-op compaction hook.
type NoopCompactor struct{}

func (NoopCompactor) CompactSource(_ context.Context, sourceID string) (CompactionResult, error) {
	return CompactionResult{
		SourceID:  sourceID,
		Compacted: false,
		Note:      "noop",
	}, nil
}
