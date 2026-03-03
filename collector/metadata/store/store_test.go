package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryStoreContract(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		t.Helper()
		return NewMemoryStore()
	})
}

func TestSQLiteStoreContract(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		t.Helper()
		dbPath := filepath.Join(t.TempDir(), "metadata.db")
		s, err := NewSQLiteStore(dbPath)
		if err != nil {
			t.Fatalf("new sqlite store failed: %v", err)
		}
		return s
	})
}

func runStoreContract(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store failed: %v", err)
		}
	})

	if _, err := s.GetSource(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing source, got %v", err)
	}
	if _, err := s.GetStream(ctx, "src-a", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing stream, got %v", err)
	}
	if _, err := s.GetArtifact(ctx, "src-a", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing artifact, got %v", err)
	}

	if err := s.UpsertSource(ctx, SourceRecord{}); err == nil {
		t.Fatalf("expected source validation error")
	}
	if err := s.UpsertStream(ctx, StreamRecord{SourceID: "src-a"}); err == nil {
		t.Fatalf("expected stream validation error")
	}
	if err := s.UpsertArtifact(ctx, ArtifactRecord{SourceID: "src-a"}); err == nil {
		t.Fatalf("expected artifact validation error")
	}

	now := time.Now().UTC().Round(time.Nanosecond)
	sourceA := SourceRecord{
		SourceID:   "src-a",
		GroupID:    "prod-a",
		Tags:       []string{"API", "api", " core "},
		Status:     "active",
		LastSeenAt: now,
		UpdatedAt:  now,
	}
	sourceB := SourceRecord{SourceID: "src-b", Status: "stale", UpdatedAt: now.Add(time.Second)}
	if err := s.UpsertSource(ctx, sourceB); err != nil {
		t.Fatalf("upsert source-b failed: %v", err)
	}
	if err := s.UpsertSource(ctx, sourceA); err != nil {
		t.Fatalf("upsert source-a failed: %v", err)
	}

	gotSource, err := s.GetSource(ctx, "src-a")
	if err != nil {
		t.Fatalf("get source-a failed: %v", err)
	}
	if gotSource.SourceID != sourceA.SourceID || gotSource.Status != sourceA.Status || !gotSource.LastSeenAt.Equal(sourceA.LastSeenAt) || !gotSource.UpdatedAt.Equal(sourceA.UpdatedAt) {
		t.Fatalf("unexpected source-a record: %+v", gotSource)
	}
	if gotSource.GroupID != "prod-a" || len(gotSource.Tags) != 2 || gotSource.Tags[0] != "api" || gotSource.Tags[1] != "core" {
		t.Fatalf("unexpected source-a grouping/tags: %+v", gotSource)
	}

	sources, err := s.ListSources(ctx)
	if err != nil {
		t.Fatalf("list sources failed: %v", err)
	}
	if len(sources) != 2 || sources[0].SourceID != "src-a" || sources[1].SourceID != "src-b" {
		t.Fatalf("unexpected source listing: %+v", sources)
	}

	stream1 := StreamRecord{SourceID: "src-a", StreamID: "stream-2", LogicalPath: "/b.log", Status: "closed", UpdatedAt: now}
	stream2 := StreamRecord{SourceID: "src-a", StreamID: "stream-1", LogicalPath: "/a.log", Status: "open", UpdatedAt: now.Add(2 * time.Second)}
	if err := s.UpsertStream(ctx, stream1); err != nil {
		t.Fatalf("upsert stream-2 failed: %v", err)
	}
	if err := s.UpsertStream(ctx, stream2); err != nil {
		t.Fatalf("upsert stream-1 failed: %v", err)
	}

	gotStream, err := s.GetStream(ctx, "src-a", "stream-1")
	if err != nil {
		t.Fatalf("get stream-1 failed: %v", err)
	}
	if gotStream.StreamID != stream2.StreamID || gotStream.Status != stream2.Status || !gotStream.UpdatedAt.Equal(stream2.UpdatedAt) {
		t.Fatalf("unexpected stream record: %+v", gotStream)
	}

	streams, err := s.ListStreamsBySource(ctx, "src-a")
	if err != nil {
		t.Fatalf("list streams failed: %v", err)
	}
	if len(streams) != 2 || streams[0].StreamID != "stream-1" || streams[1].StreamID != "stream-2" {
		t.Fatalf("unexpected stream listing: %+v", streams)
	}

	artifact1 := ArtifactRecord{
		SourceID:    "src-a",
		ArtifactID:  "artifact-2",
		Status:      "completed",
		SizeBytes:   10,
		Checksum:    "sha256:b",
		CompletedAt: now,
		UpdatedAt:   now,
	}
	artifact2 := ArtifactRecord{
		SourceID:    "src-a",
		ArtifactID:  "artifact-1",
		Status:      "open",
		SizeBytes:   5,
		Checksum:    "sha256:a",
		CompletedAt: time.Time{},
		UpdatedAt:   now.Add(3 * time.Second),
	}
	if err := s.UpsertArtifact(ctx, artifact1); err != nil {
		t.Fatalf("upsert artifact-2 failed: %v", err)
	}
	if err := s.UpsertArtifact(ctx, artifact2); err != nil {
		t.Fatalf("upsert artifact-1 failed: %v", err)
	}

	gotArtifact, err := s.GetArtifact(ctx, "src-a", "artifact-2")
	if err != nil {
		t.Fatalf("get artifact-2 failed: %v", err)
	}
	if gotArtifact.ArtifactID != artifact1.ArtifactID || gotArtifact.SizeBytes != artifact1.SizeBytes || !gotArtifact.CompletedAt.Equal(artifact1.CompletedAt) {
		t.Fatalf("unexpected artifact record: %+v", gotArtifact)
	}

	artifacts, err := s.ListArtifactsBySource(ctx, "src-a")
	if err != nil {
		t.Fatalf("list artifacts failed: %v", err)
	}
	if len(artifacts) != 2 || artifacts[0].ArtifactID != "artifact-1" || artifacts[1].ArtifactID != "artifact-2" {
		t.Fatalf("unexpected artifact listing: %+v", artifacts)
	}

	updated := sourceA
	updated.Status = "revoked"
	updated.GroupID = "prod-b"
	updated.Tags = []string{"tier-1", "tier-1", "backend"}
	updated.UpdatedAt = now.Add(5 * time.Second)
	if err := s.UpsertSource(ctx, updated); err != nil {
		t.Fatalf("upsert updated source failed: %v", err)
	}
	gotUpdated, err := s.GetSource(ctx, "src-a")
	if err != nil {
		t.Fatalf("get updated source failed: %v", err)
	}
	if gotUpdated.Status != "revoked" || !gotUpdated.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("unexpected updated source: %+v", gotUpdated)
	}
	if gotUpdated.GroupID != "prod-b" || len(gotUpdated.Tags) != 2 || gotUpdated.Tags[0] != "backend" || gotUpdated.Tags[1] != "tier-1" {
		t.Fatalf("unexpected updated source tags/group: %+v", gotUpdated)
	}
}
