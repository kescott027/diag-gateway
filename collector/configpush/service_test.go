package configpush

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/audittrail"
)

type stubAudit struct {
	mu     sync.Mutex
	events []audittrail.Event
}

func (s *stubAudit) Append(event audittrail.Event) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *stubAudit) Events() []audittrail.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]audittrail.Event, len(s.events))
	copy(out, s.events)
	return out
}

func TestSubmitApplyFailLifecycle(t *testing.T) {
	audit := &stubAudit{}
	svc := NewService(10, audit)
	now := time.Date(2026, 3, 3, 17, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	update, err := svc.Submit(SubmitRequest{
		SourceID:    "source-a",
		ConfigHash:  "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		SubmittedBy: "op-a",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if update.Status != StatusPending {
		t.Fatalf("expected pending status, got %+v", update)
	}

	now = now.Add(time.Second)
	applied, err := svc.MarkApplied(update.ID, "agent-a")
	if err != nil {
		t.Fatalf("mark applied failed: %v", err)
	}
	if applied.Status != StatusApplied || applied.AppliedAt.IsZero() {
		t.Fatalf("expected applied update, got %+v", applied)
	}
	if _, err := svc.MarkFailed(update.ID, "agent-a", "bad"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition after applied, got %v", err)
	}

	now = now.Add(time.Second)
	update2, err := svc.Submit(SubmitRequest{
		SourceID:    "source-a",
		ConfigHash:  "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		SubmittedBy: "op-a",
	})
	if err != nil {
		t.Fatalf("submit second failed: %v", err)
	}
	now = now.Add(time.Second)
	failed, err := svc.MarkFailed(update2.ID, "agent-a", "validation error")
	if err != nil {
		t.Fatalf("mark failed failed: %v", err)
	}
	if failed.Status != StatusFailed || failed.FailureReason == "" {
		t.Fatalf("expected failed status with reason, got %+v", failed)
	}

	events := audit.Events()
	if len(events) != 4 {
		t.Fatalf("expected 4 audit events, got %d", len(events))
	}
	if events[0].Action != "config.submit" || events[1].Action != "config.apply" || events[3].Action != "config.fail" {
		t.Fatalf("unexpected audit event order: %+v", events)
	}
}

func TestValidationFilterAndEviction(t *testing.T) {
	svc := NewService(2, nil)
	now := time.Date(2026, 3, 3, 17, 10, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if _, err := svc.Submit(SubmitRequest{SourceID: "bad/source", ConfigHash: "sha256:00"}); !errors.Is(err, ErrInvalidSourceID) {
		t.Fatalf("expected invalid source id error, got %v", err)
	}
	if _, err := svc.Submit(SubmitRequest{SourceID: "source-a", ConfigHash: "not-a-hash"}); !errors.Is(err, ErrInvalidConfigHash) {
		t.Fatalf("expected invalid hash error, got %v", err)
	}

	hashes := []string{
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	for _, hash := range hashes {
		if _, err := svc.Submit(SubmitRequest{SourceID: "source-a", ConfigHash: hash, SubmittedBy: "op"}); err != nil {
			t.Fatalf("submit failed for hash %s: %v", hash, err)
		}
		now = now.Add(time.Second)
	}

	all := svc.List(Filter{})
	if len(all) != 2 {
		t.Fatalf("expected eviction to keep 2 updates, got %d", len(all))
	}
	if all[0].ConfigHash != hashes[1] || all[1].ConfigHash != hashes[2] {
		t.Fatalf("expected latest hashes retained, got %+v", all)
	}

	status := StatusPending
	filtered := svc.List(Filter{SourceID: "source-a", Status: &status, Limit: 1})
	if len(filtered) != 1 {
		t.Fatalf("expected one filtered result, got %+v", filtered)
	}
}
