package upload

import (
	"errors"
	"testing"
)

func TestArtifactLifecycleBeginAppendFinalize(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(tmp)

	meta, err := svc.Begin("source-1", "artifact-1", BeginOptions{FileName: "dump.bin", ContentType: "application/octet-stream"})
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if meta.Status != "open" {
		t.Fatalf("expected open status, got %+v", meta)
	}

	next, err := svc.Append("source-1", "artifact-1", 0, []byte("abc"))
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if next != 3 {
		t.Fatalf("expected next offset 3, got %d", next)
	}

	next, err = svc.Append("source-1", "artifact-1", next, []byte("123"))
	if err != nil {
		t.Fatalf("second append failed: %v", err)
	}
	if next != 6 {
		t.Fatalf("expected next offset 6, got %d", next)
	}

	final, err := svc.Finalize("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}
	if final.Status != "completed" || final.SizeBytes != 6 || final.SHA256 == "" {
		t.Fatalf("unexpected final metadata: %+v", final)
	}

	reloaded, err := svc.LoadMetadata("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("load metadata failed: %v", err)
	}
	if reloaded.Status != "completed" || reloaded.SHA256 != final.SHA256 {
		t.Fatalf("unexpected reloaded metadata: %+v", reloaded)
	}
}

func TestAppendOffsetMismatch(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(tmp)

	if _, err := svc.Begin("source-1", "artifact-1", BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if _, err := svc.Append("source-1", "artifact-1", 0, []byte("abc")); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	if _, err := svc.Append("source-1", "artifact-1", 0, []byte("x")); !errors.Is(err, ErrOffsetMismatch) {
		t.Fatalf("expected offset mismatch, got %v", err)
	}
}

func TestAppendIsRestartSafe(t *testing.T) {
	tmp := t.TempDir()
	svc1 := NewService(tmp)
	if _, err := svc1.Begin("source-1", "artifact-1", BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	next, err := svc1.Append("source-1", "artifact-1", 0, []byte("abc"))
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	svc2 := NewService(tmp)
	next, err = svc2.Append("source-1", "artifact-1", next, []byte("def"))
	if err != nil {
		t.Fatalf("restart append failed: %v", err)
	}
	if next != 6 {
		t.Fatalf("expected next offset 6, got %d", next)
	}

	final, err := svc2.Finalize("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}
	if final.SizeBytes != 6 || final.Status != "completed" {
		t.Fatalf("unexpected final metadata: %+v", final)
	}
}

func TestInvalidIDsRejected(t *testing.T) {
	svc := NewService(t.TempDir())
	if _, err := svc.Begin("bad/source", "artifact-1", BeginOptions{}); err == nil {
		t.Fatalf("expected invalid source id error")
	}
	if _, err := svc.Begin("source-1", "bad/artifact", BeginOptions{}); err == nil {
		t.Fatalf("expected invalid artifact id error")
	}
}

func TestResumeStateLifecycle(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(tmp)

	state, err := svc.ResumeState("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("resume state before begin failed: %v", err)
	}
	if state.Status != "new" || !state.CanResume || state.NextOffset != 0 {
		t.Fatalf("unexpected pre-begin resume state: %+v", state)
	}

	if _, err := svc.Begin("source-1", "artifact-1", BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	next, err := svc.Append("source-1", "artifact-1", 0, []byte("abc"))
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if next != 3 {
		t.Fatalf("unexpected next offset: %d", next)
	}

	state, err = svc.ResumeState("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("resume state after append failed: %v", err)
	}
	if state.Status != "open" || !state.CanResume || state.NextOffset != 3 {
		t.Fatalf("unexpected open resume state: %+v", state)
	}

	if _, err := svc.Finalize("source-1", "artifact-1"); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}
	state, err = svc.ResumeState("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("resume state after finalize failed: %v", err)
	}
	if state.Status != "completed" || state.CanResume {
		t.Fatalf("unexpected completed resume state: %+v", state)
	}
}

func TestAppendAfterFinalizeRejected(t *testing.T) {
	svc := NewService(t.TempDir())
	if _, err := svc.Begin("source-1", "artifact-1", BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if _, err := svc.Append("source-1", "artifact-1", 0, []byte("abc")); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := svc.Finalize("source-1", "artifact-1"); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if _, err := svc.Append("source-1", "artifact-1", 3, []byte("x")); !errors.Is(err, ErrArtifactCompleted) {
		t.Fatalf("expected ErrArtifactCompleted, got %v", err)
	}
}
