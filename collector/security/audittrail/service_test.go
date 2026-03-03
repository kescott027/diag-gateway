package audittrail

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndQueryFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "events.log")
	svc := NewService(path)
	now := time.Date(2026, 3, 3, 16, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if err := svc.RecordEnrollment("op-a", "source-a", "success", "", map[string]any{"token_id": "tok-1"}); err != nil {
		t.Fatalf("record enrollment failed: %v", err)
	}
	now = now.Add(time.Second)
	if err := svc.RecordConfigChange("op-a", "/etc/diag/config.yaml", "success", "", map[string]any{"version": 2}); err != nil {
		t.Fatalf("record config change failed: %v", err)
	}
	now = now.Add(time.Second)
	if err := svc.RecordEnrollment("op-b", "source-b", "denied", "revoked", nil); err != nil {
		t.Fatalf("record second enrollment failed: %v", err)
	}

	all, err := svc.Query(Filter{})
	if err != nil {
		t.Fatalf("query all failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all))
	}

	byActor, err := svc.Query(Filter{Actor: "op-a"})
	if err != nil {
		t.Fatalf("query by actor failed: %v", err)
	}
	if len(byActor) != 2 {
		t.Fatalf("expected 2 op-a events, got %d", len(byActor))
	}

	byActionPrefix, err := svc.Query(Filter{ActionPrefix: "enrollment"})
	if err != nil {
		t.Fatalf("query by action prefix failed: %v", err)
	}
	if len(byActionPrefix) != 2 {
		t.Fatalf("expected 2 enrollment events, got %d", len(byActionPrefix))
	}

	bySource, err := svc.Query(Filter{SourceID: "source-b"})
	if err != nil {
		t.Fatalf("query by source failed: %v", err)
	}
	if len(bySource) != 1 || bySource[0].Actor != "op-b" {
		t.Fatalf("unexpected source filter result: %+v", bySource)
	}
}

func TestQueryLimitAndRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "events.log")
	svc := NewService(path)
	now := time.Date(2026, 3, 3, 16, 10, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		if err := svc.Append(Event{Actor: "op", Action: "config.update"}); err != nil {
			t.Fatalf("append event %d failed: %v", i, err)
		}
		now = now.Add(time.Second)
	}

	limited, err := svc.Query(Filter{Limit: 2})
	if err != nil {
		t.Fatalf("query limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("expected limit 2, got %d", len(limited))
	}

	rangeQuery, err := svc.Query(Filter{
		Since: time.Date(2026, 3, 3, 16, 10, 2, 0, time.UTC),
		Until: time.Date(2026, 3, 3, 16, 10, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("query range failed: %v", err)
	}
	if len(rangeQuery) != 2 {
		t.Fatalf("expected 2 events in time window, got %d", len(rangeQuery))
	}
}

func TestAppendRequiresAction(t *testing.T) {
	svc := NewService(filepath.Join(t.TempDir(), "audit", "events.log"))
	if err := svc.Append(Event{Actor: "op"}); err == nil {
		t.Fatalf("expected missing action error")
	}
}
