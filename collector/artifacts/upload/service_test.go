package upload

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	if final.Status != "completed" || final.SizeBytes != 6 || final.SHA256 == "" || final.Checksum != final.SHA256 {
		t.Fatalf("unexpected final metadata: %+v", final)
	}
	if final.Name != "dump.bin" || final.UploadTimestamp == "" {
		t.Fatalf("expected enriched metadata fields, got %+v", final)
	}

	reloaded, err := svc.LoadMetadata("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("load metadata failed: %v", err)
	}
	if reloaded.Status != "completed" || reloaded.SHA256 != final.SHA256 {
		t.Fatalf("unexpected reloaded metadata: %+v", reloaded)
	}
	if reloaded.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %+v", reloaded)
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

func TestUpdateMetadataTagsAndAttributes(t *testing.T) {
	svc := NewService(t.TempDir())
	if _, err := svc.Begin("source-1", "artifact-1", BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}

	name := "trace.zip"
	contentType := "application/zip"
	updated, err := svc.UpdateMetadata("source-1", "artifact-1", MetadataUpdate{
		FileName:    &name,
		ContentType: &contentType,
		Tags:        []string{"prod", "api", "prod", "  ", "incident"},
	})
	if err != nil {
		t.Fatalf("update metadata failed: %v", err)
	}
	if updated.FileName != "trace.zip" || updated.Name != "trace.zip" {
		t.Fatalf("expected file name propagation, got %+v", updated)
	}
	if updated.ContentType != contentType {
		t.Fatalf("expected content type %q, got %+v", contentType, updated)
	}
	if len(updated.Tags) != 3 || updated.Tags[0] != "api" || updated.Tags[2] != "prod" {
		t.Fatalf("expected normalized tags, got %+v", updated.Tags)
	}
}

func TestLoadLegacyMetadataCompatibility(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(tmp)

	artifactDir := filepath.Join(tmp, "sources", "source-1", "artifacts", "artifact-1")
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatalf("mkdir artifact dir failed: %v", err)
	}
	legacy := map[string]any{
		"name":             "legacy.dump",
		"size":             42, // ignored field from legacy docs
		"size_bytes":       42,
		"checksum":         "abc123",
		"upload_timestamp": "2026-03-03T00:00:00Z",
	}
	blob, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy metadata failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "metadata.json"), blob, 0o600); err != nil {
		t.Fatalf("write legacy metadata failed: %v", err)
	}

	meta, err := svc.LoadMetadata("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("load metadata failed: %v", err)
	}
	if meta.FileName != "legacy.dump" || meta.Name != "legacy.dump" {
		t.Fatalf("expected legacy name mapping, got %+v", meta)
	}
	if meta.SHA256 != "abc123" || meta.Checksum != "abc123" {
		t.Fatalf("expected checksum mapping, got %+v", meta)
	}
	if meta.CreatedAt != "2026-03-03T00:00:00Z" || meta.UploadTimestamp != "2026-03-03T00:00:00Z" {
		t.Fatalf("expected upload timestamp mapping, got %+v", meta)
	}
}
