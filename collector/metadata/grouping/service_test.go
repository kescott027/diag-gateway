package grouping

import (
	"context"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/metadata/store"
)

func TestAssignAndFilterByGroupAndTag(t *testing.T) {
	mem := store.NewMemoryStore()
	t.Cleanup(func() { _ = mem.Close() })
	ctx := context.Background()
	now := time.Now().UTC()
	if err := mem.UpsertSource(ctx, store.SourceRecord{SourceID: "source-a", Status: "active", UpdatedAt: now}); err != nil {
		t.Fatalf("upsert source-a failed: %v", err)
	}
	if err := mem.UpsertSource(ctx, store.SourceRecord{SourceID: "source-b", Status: "active", UpdatedAt: now}); err != nil {
		t.Fatalf("upsert source-b failed: %v", err)
	}

	svc, err := NewService(mem)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	updatedA, err := svc.Assign(ctx, Assignment{
		SourceID: "source-a",
		GroupID:  "Team-1",
		Tags:     []string{"API", "api", "core"},
	})
	if err != nil {
		t.Fatalf("assign source-a failed: %v", err)
	}
	if updatedA.GroupID != "team-1" || len(updatedA.Tags) != 2 || updatedA.Tags[0] != "api" || updatedA.Tags[1] != "core" {
		t.Fatalf("unexpected source-a assignment: %+v", updatedA)
	}
	if _, err := svc.Assign(ctx, Assignment{SourceID: "source-b", GroupID: "team-1", Tags: []string{"worker"}}); err != nil {
		t.Fatalf("assign source-b failed: %v", err)
	}

	byGroup, err := svc.ListByGroup(ctx, "TEAM-1")
	if err != nil {
		t.Fatalf("list by group failed: %v", err)
	}
	if len(byGroup) != 2 || byGroup[0].SourceID != "source-a" || byGroup[1].SourceID != "source-b" {
		t.Fatalf("unexpected group filter result: %+v", byGroup)
	}

	byTag, err := svc.ListByTag(ctx, "api")
	if err != nil {
		t.Fatalf("list by tag failed: %v", err)
	}
	if len(byTag) != 1 || byTag[0].SourceID != "source-a" {
		t.Fatalf("unexpected tag filter result: %+v", byTag)
	}
}

func TestAssignCreatesSourceWhenMissing(t *testing.T) {
	mem := store.NewMemoryStore()
	t.Cleanup(func() { _ = mem.Close() })

	svc, err := NewService(mem)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	rec, err := svc.Assign(context.Background(), Assignment{
		SourceID: "new-source",
		GroupID:  "ops",
		Tags:     []string{"infra"},
	})
	if err != nil {
		t.Fatalf("assign missing source failed: %v", err)
	}
	if rec.Status != "unknown" {
		t.Fatalf("expected unknown status for created source, got %+v", rec)
	}
}

func TestAssignRejectsInvalidInputs(t *testing.T) {
	mem := store.NewMemoryStore()
	t.Cleanup(func() { _ = mem.Close() })
	svc, err := NewService(mem)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	if _, err := svc.Assign(context.Background(), Assignment{SourceID: ""}); err == nil {
		t.Fatalf("expected missing source id error")
	}
	if _, err := svc.Assign(context.Background(), Assignment{SourceID: "src-a", GroupID: "bad/group"}); err == nil {
		t.Fatalf("expected invalid group id error")
	}
	if _, err := svc.Assign(context.Background(), Assignment{SourceID: "src-a", Tags: []string{"bad tag"}}); err == nil {
		t.Fatalf("expected invalid tag error")
	}
}
