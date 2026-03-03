package exporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/artifacts/upload"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
	"github.com/kescott027/diag-gateway/collector/stream/reassembly"
)

func TestExportDeterministicBundleAndManifest(t *testing.T) {
	tmp := t.TempDir()
	seedSourceData(t, tmp)

	svc := NewService(tmp)
	bundleA := filepath.Join(tmp, "bundle-a.tar.gz")
	bundleB := filepath.Join(tmp, "bundle-b.tar.gz")

	resA, err := svc.Export(Request{SourceID: "source-1", OutputPath: bundleA})
	if err != nil {
		t.Fatalf("export A failed: %v", err)
	}
	resB, err := svc.Export(Request{SourceID: "source-1", OutputPath: bundleB})
	if err != nil {
		t.Fatalf("export B failed: %v", err)
	}

	blobA, err := os.ReadFile(bundleA)
	if err != nil {
		t.Fatalf("read bundle A failed: %v", err)
	}
	blobB, err := os.ReadFile(bundleB)
	if err != nil {
		t.Fatalf("read bundle B failed: %v", err)
	}
	if string(blobA) != string(blobB) {
		t.Fatalf("expected deterministic bundle bytes")
	}
	if resA.SHA256 != resB.SHA256 {
		t.Fatalf("expected matching bundle digest, got %s vs %s", resA.SHA256, resB.SHA256)
	}

	manifest, err := ReadTarGzManifest(bundleA)
	if err != nil {
		t.Fatalf("read bundle manifest failed: %v", err)
	}
	if manifest.SourceID != "source-1" {
		t.Fatalf("unexpected manifest source: %+v", manifest)
	}
	if len(manifest.Entries) == 0 {
		t.Fatalf("expected manifest entries")
	}

	sidecarBlob, err := os.ReadFile(resA.ManifestPath)
	if err != nil {
		t.Fatalf("read sidecar manifest failed: %v", err)
	}
	var sidecar Manifest
	if err := json.Unmarshal(sidecarBlob, &sidecar); err != nil {
		t.Fatalf("decode sidecar manifest failed: %v", err)
	}
	if len(sidecar.Entries) != len(manifest.Entries) {
		t.Fatalf("sidecar and embedded manifest mismatch")
	}
}

func TestExportWindowFiltering(t *testing.T) {
	tmp := t.TempDir()
	seedSourceData(t, tmp)

	svc := NewService(tmp)
	start := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	res, err := svc.Export(Request{
		SourceID:    "source-1",
		OutputPath:  filepath.Join(tmp, "windowed.tar.gz"),
		WindowStart: start,
		WindowEnd:   end,
	})
	if err != nil {
		t.Fatalf("windowed export failed: %v", err)
	}

	manifestBlob, err := os.ReadFile(res.ManifestPath)
	if err != nil {
		t.Fatalf("read windowed manifest failed: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBlob, &manifest); err != nil {
		t.Fatalf("decode windowed manifest failed: %v", err)
	}
	if len(manifest.Entries) == 0 {
		t.Fatalf("expected some entries in windowed bundle")
	}
	for _, entry := range manifest.Entries {
		if entry.Path == "sources/source-1/streams/old-stream/stream.log" {
			t.Fatalf("expected old stream to be filtered out")
		}
	}
}

func seedSourceData(t *testing.T, dataDir string) {
	t.Helper()

	writeStream(t, dataDir, "source-1", "recent-stream", "2026-03-03T12:00:00Z")
	writeStream(t, dataDir, "source-1", "old-stream", "2025-01-01T00:00:00Z")
	writeArtifact(t, dataDir, "source-1", "recent-artifact", "2026-03-03T12:00:00Z")
	writeArtifact(t, dataDir, "source-1", "old-artifact", "2025-01-01T00:00:00Z")
}

func writeStream(t *testing.T, dataDir, sourceID, streamID, createdAt string) {
	t.Helper()
	streamDir, err := layout.StreamDir(dataDir, sourceID, streamID)
	if err != nil {
		t.Fatalf("stream dir path failed: %v", err)
	}
	if err := os.MkdirAll(streamDir, 0o700); err != nil {
		t.Fatalf("mkdir stream dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "stream.log"), []byte("line\n"), 0o600); err != nil {
		t.Fatalf("write stream log failed: %v", err)
	}
	meta := reassembly.Metadata{LogicalPath: streamID + ".log", CreatedAt: createdAt, Status: "closed"}
	metaPath, err := layout.StreamMetadataPath(dataDir, sourceID, streamID)
	if err != nil {
		t.Fatalf("stream metadata path failed: %v", err)
	}
	blob, _ := json.Marshal(meta)
	if err := os.WriteFile(metaPath, blob, 0o600); err != nil {
		t.Fatalf("write stream metadata failed: %v", err)
	}
}

func writeArtifact(t *testing.T, dataDir, sourceID, artifactID, completedAt string) {
	t.Helper()
	artifactDir, err := layout.ArtifactDir(dataDir, sourceID, artifactID)
	if err != nil {
		t.Fatalf("artifact dir path failed: %v", err)
	}
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatalf("mkdir artifact dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "artifact.bin"), []byte("artifact"), 0o600); err != nil {
		t.Fatalf("write artifact bin failed: %v", err)
	}
	meta := upload.Metadata{SourceID: sourceID, ArtifactID: artifactID, Status: "completed", CompletedAt: completedAt}
	metaPath, err := layout.ArtifactMetadataPath(dataDir, sourceID, artifactID)
	if err != nil {
		t.Fatalf("artifact metadata path failed: %v", err)
	}
	blob, _ := json.Marshal(meta)
	if err := os.WriteFile(metaPath, blob, 0o600); err != nil {
		t.Fatalf("write artifact metadata failed: %v", err)
	}
}
