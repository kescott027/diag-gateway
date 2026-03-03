package layout

import "testing"

func TestDeterministicStreamAndArtifactPaths(t *testing.T) {
	dataDir := "/tmp/data"
	sourceID := "source-A"
	streamID := "stream-123"
	artifactID := "artifact-456"

	streamLog, err := StreamLogPath(dataDir, sourceID, streamID)
	if err != nil {
		t.Fatalf("stream log path failed: %v", err)
	}
	if streamLog != "/tmp/data/sources/source-A/streams/stream-123/stream.log" {
		t.Fatalf("unexpected stream path: %s", streamLog)
	}

	artifactBin, err := ArtifactBinPath(dataDir, sourceID, artifactID)
	if err != nil {
		t.Fatalf("artifact path failed: %v", err)
	}
	if artifactBin != "/tmp/data/sources/source-A/artifacts/artifact-456/artifact.bin" {
		t.Fatalf("unexpected artifact path: %s", artifactBin)
	}
}

func TestRejectsInvalidIDs(t *testing.T) {
	if _, err := SourceRoot("/tmp/data", "../etc/passwd"); err == nil {
		t.Fatalf("expected invalid source id rejection")
	}
	if _, err := StreamDir("/tmp/data", "source-1", "bad/stream"); err == nil {
		t.Fatalf("expected invalid stream id rejection")
	}
	if _, err := ArtifactDir("/tmp/data", "source-1", ""); err == nil {
		t.Fatalf("expected missing artifact id rejection")
	}
}
