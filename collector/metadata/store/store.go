package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("metadata record not found")

// SourceRecord represents one source metadata row.
type SourceRecord struct {
	SourceID   string    `json:"source_id"`
	GroupID    string    `json:"group_id,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Status     string    `json:"status"`
	LastSeenAt time.Time `json:"last_seen_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// StreamRecord represents one stream metadata row.
type StreamRecord struct {
	SourceID    string    `json:"source_id"`
	StreamID    string    `json:"stream_id"`
	LogicalPath string    `json:"logical_path"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ArtifactRecord represents one artifact metadata row.
type ArtifactRecord struct {
	SourceID    string    `json:"source_id"`
	ArtifactID  string    `json:"artifact_id"`
	Status      string    `json:"status"`
	SizeBytes   int64     `json:"size_bytes"`
	Checksum    string    `json:"checksum"`
	CompletedAt time.Time `json:"completed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store defines backend-neutral metadata operations.
type Store interface {
	UpsertSource(ctx context.Context, rec SourceRecord) error
	GetSource(ctx context.Context, sourceID string) (SourceRecord, error)
	ListSources(ctx context.Context) ([]SourceRecord, error)

	UpsertStream(ctx context.Context, rec StreamRecord) error
	GetStream(ctx context.Context, sourceID, streamID string) (StreamRecord, error)
	ListStreamsBySource(ctx context.Context, sourceID string) ([]StreamRecord, error)

	UpsertArtifact(ctx context.Context, rec ArtifactRecord) error
	GetArtifact(ctx context.Context, sourceID, artifactID string) (ArtifactRecord, error)
	ListArtifactsBySource(ctx context.Context, sourceID string) ([]ArtifactRecord, error)

	Close() error
}

func validateID(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func validateOptionalID(name, value string) error {
	if value == "" {
		return nil
	}
	return validateID(name, value)
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := strings.ToLower(strings.TrimSpace(tag))
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func cloneTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, len(tags))
	copy(out, tags)
	return out
}

// MemoryStore is an in-memory metadata adapter for tests and lightweight runs.
type MemoryStore struct {
	mu sync.RWMutex

	sources   map[string]SourceRecord
	streams   map[string]StreamRecord
	artifacts map[string]ArtifactRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sources:   make(map[string]SourceRecord),
		streams:   make(map[string]StreamRecord),
		artifacts: make(map[string]ArtifactRecord),
	}
}

func (m *MemoryStore) Close() error { return nil }

func (m *MemoryStore) UpsertSource(_ context.Context, rec SourceRecord) error {
	if err := validateID("source_id", rec.SourceID); err != nil {
		return err
	}
	if err := validateOptionalID("group_id", rec.GroupID); err != nil {
		return err
	}
	rec.GroupID = strings.ToLower(strings.TrimSpace(rec.GroupID))
	rec.Tags = normalizeTags(rec.Tags)
	m.mu.Lock()
	m.sources[rec.SourceID] = rec
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) GetSource(_ context.Context, sourceID string) (SourceRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return SourceRecord{}, err
	}
	m.mu.RLock()
	rec, ok := m.sources[sourceID]
	m.mu.RUnlock()
	if !ok {
		return SourceRecord{}, ErrNotFound
	}
	rec.Tags = cloneTags(rec.Tags)
	return rec, nil
}

func (m *MemoryStore) ListSources(_ context.Context) ([]SourceRecord, error) {
	m.mu.RLock()
	out := make([]SourceRecord, 0, len(m.sources))
	for _, rec := range m.sources {
		rec.Tags = cloneTags(rec.Tags)
		out = append(out, rec)
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].SourceID < out[j].SourceID })
	return out, nil
}

func streamKey(sourceID, streamID string) string     { return sourceID + "/" + streamID }
func artifactKey(sourceID, artifactID string) string { return sourceID + "/" + artifactID }

func (m *MemoryStore) UpsertStream(_ context.Context, rec StreamRecord) error {
	if err := validateID("source_id", rec.SourceID); err != nil {
		return err
	}
	if err := validateID("stream_id", rec.StreamID); err != nil {
		return err
	}
	m.mu.Lock()
	m.streams[streamKey(rec.SourceID, rec.StreamID)] = rec
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) GetStream(_ context.Context, sourceID, streamID string) (StreamRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return StreamRecord{}, err
	}
	if err := validateID("stream_id", streamID); err != nil {
		return StreamRecord{}, err
	}
	m.mu.RLock()
	rec, ok := m.streams[streamKey(sourceID, streamID)]
	m.mu.RUnlock()
	if !ok {
		return StreamRecord{}, ErrNotFound
	}
	return rec, nil
}

func (m *MemoryStore) ListStreamsBySource(_ context.Context, sourceID string) ([]StreamRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return nil, err
	}
	m.mu.RLock()
	out := make([]StreamRecord, 0)
	for _, rec := range m.streams {
		if rec.SourceID == sourceID {
			out = append(out, rec)
		}
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].StreamID < out[j].StreamID })
	return out, nil
}

func (m *MemoryStore) UpsertArtifact(_ context.Context, rec ArtifactRecord) error {
	if err := validateID("source_id", rec.SourceID); err != nil {
		return err
	}
	if err := validateID("artifact_id", rec.ArtifactID); err != nil {
		return err
	}
	m.mu.Lock()
	m.artifacts[artifactKey(rec.SourceID, rec.ArtifactID)] = rec
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) GetArtifact(_ context.Context, sourceID, artifactID string) (ArtifactRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return ArtifactRecord{}, err
	}
	if err := validateID("artifact_id", artifactID); err != nil {
		return ArtifactRecord{}, err
	}
	m.mu.RLock()
	rec, ok := m.artifacts[artifactKey(sourceID, artifactID)]
	m.mu.RUnlock()
	if !ok {
		return ArtifactRecord{}, ErrNotFound
	}
	return rec, nil
}

func (m *MemoryStore) ListArtifactsBySource(_ context.Context, sourceID string) ([]ArtifactRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return nil, err
	}
	m.mu.RLock()
	out := make([]ArtifactRecord, 0)
	for _, rec := range m.artifacts {
		if rec.SourceID == sourceID {
			out = append(out, rec)
		}
	}
	m.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ArtifactID < out[j].ArtifactID })
	return out, nil
}
