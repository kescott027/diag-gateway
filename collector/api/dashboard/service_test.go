package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/api/listing"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
	"github.com/kescott027/diag-gateway/collector/telemetry/agentmetrics"
	"github.com/kescott027/diag-gateway/collector/telemetry/lastseen"
)

type fakeLastSeen struct {
	records []lastseen.Record
}

func (f fakeLastSeen) Snapshot(_ time.Duration) []lastseen.Record {
	out := make([]lastseen.Record, len(f.records))
	copy(out, f.records)
	return out
}

type fakeMetrics struct {
	records []agentmetrics.Record
}

func (f fakeMetrics) Snapshot() []agentmetrics.Record {
	out := make([]agentmetrics.Record, len(f.records))
	copy(out, f.records)
	return out
}

func TestCardsAggregatesSourcesDeterministically(t *testing.T) {
	tmp := t.TempDir()
	seedStreamsAndArtifacts(t, tmp)

	svc := NewService(tmp, time.Minute,
		fakeLastSeen{records: []lastseen.Record{{SourceID: "source-b", Status: lastseen.StatusStale, LastSeenAt: time.Unix(100, 0), AgeSeconds: 9}, {SourceID: "source-a", Status: lastseen.StatusActive, LastSeenAt: time.Unix(200, 0), AgeSeconds: 1}}},
		fakeMetrics{records: []agentmetrics.Record{{SourceID: "source-a", TotalBytes: 42, TotalErrors: 2, QueueDepth: 1, ThroughputBytesPerSecond: 1.5, ErrorRatePerSecond: 0.1}}},
	)

	cards, err := svc.Cards(Query{})
	if err != nil {
		t.Fatalf("cards failed: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %+v", cards)
	}
	if cards[0].SourceID != "source-a" || cards[1].SourceID != "source-b" {
		t.Fatalf("expected source sort order, got %+v", cards)
	}
	if cards[0].StreamCount != 2 || cards[0].ArtifactCount != 1 {
		t.Fatalf("unexpected source-a counts: %+v", cards[0])
	}
	if cards[0].TotalBytes != 42 || cards[0].TotalErrors != 2 || cards[0].Status != "active" {
		t.Fatalf("unexpected source-a telemetry fields: %+v", cards[0])
	}
	if cards[1].Status != "stale" {
		t.Fatalf("expected stale source-b status, got %+v", cards[1])
	}
}

func TestCardsFilterBySource(t *testing.T) {
	tmp := t.TempDir()
	seedStreamsAndArtifacts(t, tmp)

	svc := NewService(tmp, time.Minute, nil, nil)
	cards, err := svc.Cards(Query{SourceIDs: []string{"source-b"}})
	if err != nil {
		t.Fatalf("cards filter failed: %v", err)
	}
	if len(cards) != 1 || cards[0].SourceID != "source-b" {
		t.Fatalf("expected one source-b card, got %+v", cards)
	}
}

func TestCardsInvalidFilterSourceID(t *testing.T) {
	svc := NewService(t.TempDir(), time.Minute, nil, nil)
	if _, err := svc.Cards(Query{SourceIDs: []string{"bad/source"}}); err == nil {
		t.Fatalf("expected invalid source id error")
	}
}

func seedStreamsAndArtifacts(t *testing.T, dataDir string) {
	t.Helper()

	writeStreamMeta(t, dataDir, "source-a", "stream-1", listing.FileEntry{LogicalPath: "a.log", LastOffset: 10, Status: "closed"})
	writeStreamMeta(t, dataDir, "source-a", "stream-2", listing.FileEntry{LogicalPath: "b.log", LastOffset: 20, Status: "closed"})
	writeArtifactDir(t, dataDir, "source-a", "artifact-1")

	writeStreamMeta(t, dataDir, "source-b", "stream-1", listing.FileEntry{LogicalPath: "c.log", LastOffset: 30, Status: "open"})
	writeArtifactDir(t, dataDir, "source-b", "artifact-1")
	writeArtifactDir(t, dataDir, "source-b", "artifact-2")
}

func writeStreamMeta(t *testing.T, dataDir, sourceID, streamID string, entry listing.FileEntry) {
	t.Helper()
	streamDir, err := layout.StreamDir(dataDir, sourceID, streamID)
	if err != nil {
		t.Fatalf("stream dir path failed: %v", err)
	}
	if err := os.MkdirAll(streamDir, 0o700); err != nil {
		t.Fatalf("mkdir stream dir failed: %v", err)
	}
	metaPath, err := layout.StreamMetadataPath(dataDir, sourceID, streamID)
	if err != nil {
		t.Fatalf("stream metadata path failed: %v", err)
	}
	metaBlob, _ := json.Marshal(map[string]any{"logical_path": entry.LogicalPath, "last_offset": entry.LastOffset, "status": entry.Status})
	if err := os.WriteFile(metaPath, metaBlob, 0o600); err != nil {
		t.Fatalf("write stream metadata failed: %v", err)
	}
}

func writeArtifactDir(t *testing.T, dataDir, sourceID, artifactID string) {
	t.Helper()
	artifactDir, err := layout.ArtifactDir(dataDir, sourceID, artifactID)
	if err != nil {
		t.Fatalf("artifact dir path failed: %v", err)
	}
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatalf("mkdir artifact dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "metadata.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write artifact metadata failed: %v", err)
	}
}
