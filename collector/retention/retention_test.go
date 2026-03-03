package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/artifacts/upload"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
	"github.com/kescott027/diag-gateway/collector/stream/reassembly"
)

func TestPolicySetAndGet(t *testing.T) {
	svc := NewService(t.TempDir())
	svc.now = func() time.Time { return time.Unix(1000, 0).UTC() }

	set, err := svc.SetPolicy(Policy{SourceID: "source-1", StreamRetentionDays: 7, ArtifactRetentionDays: 14})
	if err != nil {
		t.Fatalf("set policy failed: %v", err)
	}
	if set.UpdatedAt == "" {
		t.Fatalf("expected updated_at to be set")
	}

	got, err := svc.GetPolicy("source-1")
	if err != nil {
		t.Fatalf("get policy failed: %v", err)
	}
	if got.StreamRetentionDays != 7 || got.ArtifactRetentionDays != 14 {
		t.Fatalf("unexpected policy: %+v", got)
	}
}

func TestPruneSourceDeletesOldCompletedArtifactsAndStreams(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(tmp)
	now := time.Date(2026, 3, 3, 14, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	if _, err := svc.SetPolicy(Policy{SourceID: "source-1", StreamRetentionDays: 7, ArtifactRetentionDays: 7}); err != nil {
		t.Fatalf("set policy failed: %v", err)
	}

	writeStreamMeta(t, tmp, "source-1", "old-closed", reassembly.Metadata{
		CreatedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		Status:    "closed",
	})
	writeStreamMeta(t, tmp, "source-1", "new-closed", reassembly.Metadata{
		CreatedAt: now.Add(-2 * 24 * time.Hour).Format(time.RFC3339),
		Status:    "closed",
	})
	writeStreamMeta(t, tmp, "source-1", "old-open", reassembly.Metadata{
		CreatedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		Status:    "open",
	})

	writeArtifactMeta(t, tmp, "source-1", "old-completed", upload.Metadata{
		Status:      "completed",
		CompletedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
	})
	writeArtifactMeta(t, tmp, "source-1", "new-completed", upload.Metadata{
		Status:      "completed",
		CompletedAt: now.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
	})
	writeArtifactMeta(t, tmp, "source-1", "old-open", upload.Metadata{
		Status:    "open",
		CreatedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
	})

	res, err := svc.PruneSource("source-1")
	if err != nil {
		t.Fatalf("prune source failed: %v", err)
	}
	if res.StreamsDeleted != 1 || res.StreamsSkippedActive != 1 {
		t.Fatalf("unexpected stream prune result: %+v", res)
	}
	if res.ArtifactsDeleted != 1 || res.ArtifactsSkippedOpen != 1 {
		t.Fatalf("unexpected artifact prune result: %+v", res)
	}

	mustNotExist(t, filepath.Join(tmp, "sources", "source-1", "streams", "old-closed"))
	mustExist(t, filepath.Join(tmp, "sources", "source-1", "streams", "new-closed"))
	mustExist(t, filepath.Join(tmp, "sources", "source-1", "streams", "old-open"))

	mustNotExist(t, filepath.Join(tmp, "sources", "source-1", "artifacts", "old-completed"))
	mustExist(t, filepath.Join(tmp, "sources", "source-1", "artifacts", "new-completed"))
	mustExist(t, filepath.Join(tmp, "sources", "source-1", "artifacts", "old-open"))
}

func TestPruneSourceNoPolicyNoDeletion(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService(tmp)
	writeStreamMeta(t, tmp, "source-1", "closed", reassembly.Metadata{CreatedAt: time.Now().Add(-48 * time.Hour).Format(time.RFC3339), Status: "closed"})

	res, err := svc.PruneSource("source-1")
	if err != nil {
		t.Fatalf("prune source failed: %v", err)
	}
	if res.StreamsDeleted != 0 || res.ArtifactsDeleted != 0 {
		t.Fatalf("expected no deletion without policy, got %+v", res)
	}
}

func writeStreamMeta(t *testing.T, dataDir, sourceID, streamID string, meta reassembly.Metadata) {
	t.Helper()
	streamDir, err := layout.StreamDir(dataDir, sourceID, streamID)
	if err != nil {
		t.Fatalf("stream dir path failed: %v", err)
	}
	if err := os.MkdirAll(streamDir, 0o700); err != nil {
		t.Fatalf("mkdir stream dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "stream.log"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write stream log failed: %v", err)
	}
	metaPath, err := layout.StreamMetadataPath(dataDir, sourceID, streamID)
	if err != nil {
		t.Fatalf("stream metadata path failed: %v", err)
	}
	if err := writeAtomicJSON(metaPath, meta); err != nil {
		t.Fatalf("write stream metadata failed: %v", err)
	}
}

func writeArtifactMeta(t *testing.T, dataDir, sourceID, artifactID string, meta upload.Metadata) {
	t.Helper()
	artifactDir, err := layout.ArtifactDir(dataDir, sourceID, artifactID)
	if err != nil {
		t.Fatalf("artifact dir path failed: %v", err)
	}
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatalf("mkdir artifact dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "artifact.bin"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write artifact bin failed: %v", err)
	}
	metaPath, err := layout.ArtifactMetadataPath(dataDir, sourceID, artifactID)
	if err != nil {
		t.Fatalf("artifact metadata path failed: %v", err)
	}
	if err := writeAtomicJSON(metaPath, meta); err != nil {
		t.Fatalf("write artifact metadata failed: %v", err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist %s, got %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path to be deleted %s, got %v", path, err)
	}
}
