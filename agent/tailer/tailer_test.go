package tailer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/agent/cursor"
	"github.com/kescott027/diag-gateway/agent/rotation"
)

func TestPollOnceReadsOnlyAppendedData(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logPath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	store, err := cursor.NewStore(filepath.Join(dir, "cursors.json"))
	if err != nil {
		t.Fatalf("new cursor store failed: %v", err)
	}

	var chunks []Chunk
	tlr, err := NewTailer(Config{
		SourceID:  "source-1",
		FileKey:   "app.log",
		Path:      logPath,
		ChunkSize: 2,
	}, store, func(c Chunk) error {
		chunks = append(chunks, c)
		return nil
	})
	if err != nil {
		t.Fatalf("new tailer failed: %v", err)
	}

	res1, err := tlr.PollOnce()
	if err != nil {
		t.Fatalf("poll once failed: %v", err)
	}
	if res1.BytesRead != 3 || res1.Chunks != 2 {
		t.Fatalf("unexpected first poll result: %+v", res1)
	}
	if !res1.Restarted || res1.Action != rotation.ActionStart {
		t.Fatalf("expected initial start action, got %+v", res1)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open append failed: %v", err)
	}
	if _, err := f.WriteString("de"); err != nil {
		_ = f.Close()
		t.Fatalf("append failed: %v", err)
	}
	_ = f.Close()
	time.Sleep(5 * time.Millisecond)

	res2, err := tlr.PollOnce()
	if err != nil {
		t.Fatalf("second poll failed: %v", err)
	}
	if res2.BytesRead != 2 || res2.Chunks != 1 {
		t.Fatalf("unexpected second poll result: %+v", res2)
	}
	if res2.Restarted || res2.Action != rotation.ActionResume {
		t.Fatalf("expected resume action, got %+v", res2)
	}
	last := chunks[len(chunks)-1]
	if string(last.Payload) != "de" || last.Offset != 3 {
		t.Fatalf("unexpected appended chunk: %+v", last)
	}
}

func TestPollOnceDetectsTruncationAndReopens(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "service.log")
	if err := os.WriteFile(logPath, []byte("abcdef"), 0o600); err != nil {
		t.Fatalf("write log failed: %v", err)
	}
	store, err := cursor.NewStore(filepath.Join(dir, "cursors.json"))
	if err != nil {
		t.Fatalf("new cursor store failed: %v", err)
	}

	var chunks []Chunk
	tlr, err := NewTailer(Config{
		SourceID:  "source-1",
		FileKey:   "service.log",
		Path:      logPath,
		ChunkSize: 16,
	}, store, func(c Chunk) error {
		chunks = append(chunks, c)
		return nil
	})
	if err != nil {
		t.Fatalf("new tailer failed: %v", err)
	}

	if _, err := tlr.PollOnce(); err != nil {
		t.Fatalf("initial poll failed: %v", err)
	}

	if err := os.WriteFile(logPath, []byte("xy"), 0o600); err != nil {
		t.Fatalf("truncate+write failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	res, err := tlr.PollOnce()
	if err != nil {
		t.Fatalf("truncation poll failed: %v", err)
	}
	if !res.Restarted || res.Action != rotation.ActionReopenTruncated {
		t.Fatalf("expected truncation reopen, got %+v", res)
	}
	last := chunks[len(chunks)-1]
	if !last.StreamRestart || string(last.Payload) != "xy" || last.Offset != 0 {
		t.Fatalf("unexpected truncation chunk: %+v", last)
	}
}

func TestPollOnceDetectsRotationOrRollbackReopen(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "rotate.log")
	if err := os.WriteFile(logPath, []byte("old-data"), 0o600); err != nil {
		t.Fatalf("write log failed: %v", err)
	}
	store, err := cursor.NewStore(filepath.Join(dir, "cursors.json"))
	if err != nil {
		t.Fatalf("new cursor store failed: %v", err)
	}

	var chunks []Chunk
	tlr, err := NewTailer(Config{
		SourceID:  "source-1",
		FileKey:   "rotate.log",
		Path:      logPath,
		ChunkSize: 16,
	}, store, func(c Chunk) error {
		chunks = append(chunks, c)
		return nil
	})
	if err != nil {
		t.Fatalf("new tailer failed: %v", err)
	}

	if _, err := tlr.PollOnce(); err != nil {
		t.Fatalf("initial poll failed: %v", err)
	}

	rotatedPath := logPath + ".1"
	if err := os.Rename(logPath, rotatedPath); err != nil {
		t.Fatalf("rename old log failed: %v", err)
	}
	if err := os.WriteFile(logPath, []byte("new"), 0o600); err != nil {
		t.Fatalf("write rotated log failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	res, err := tlr.PollOnce()
	if err != nil {
		t.Fatalf("rotation poll failed: %v", err)
	}
	if !res.Restarted {
		t.Fatalf("expected restart on rotation/rollback, got %+v", res)
	}
	if res.Action != rotation.ActionReopenRotated && res.Action != rotation.ActionReopenTruncated {
		t.Fatalf("unexpected restart action: %+v", res)
	}
	last := chunks[len(chunks)-1]
	if !last.StreamRestart || string(last.Payload) != "new" {
		t.Fatalf("unexpected rotated chunk: %+v", last)
	}
}

func TestPollOnceMaxFileSizeControls(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "size.log")
	store, err := cursor.NewStore(filepath.Join(dir, "cursors.json"))
	if err != nil {
		t.Fatalf("new cursor store failed: %v", err)
	}

	makeTailer := func(maxMB int) *Tailer {
		tlr, err := NewTailer(Config{
			SourceID:      "source-1",
			FileKey:       "size.log",
			Path:          logPath,
			ChunkSize:     1024,
			MaxFileSizeMB: maxMB,
		}, store, func(c Chunk) error { return nil })
		if err != nil {
			t.Fatalf("new tailer failed: %v", err)
		}
		return tlr
	}

	if err := os.WriteFile(logPath, []byte("tiny"), 0o600); err != nil {
		t.Fatalf("write tiny log failed: %v", err)
	}
	under := makeTailer(1)
	resUnder, err := under.PollOnce()
	if err != nil {
		t.Fatalf("under-threshold poll failed: %v", err)
	}
	if resUnder.Skipped {
		t.Fatalf("did not expect under-threshold file to be skipped")
	}

	oneMB := make([]byte, 1024*1024)
	if err := os.WriteFile(logPath, oneMB, 0o600); err != nil {
		t.Fatalf("write at-threshold log failed: %v", err)
	}
	at := makeTailer(1)
	resAt, err := at.PollOnce()
	if err != nil {
		t.Fatalf("at-threshold poll failed: %v", err)
	}
	if resAt.Skipped {
		t.Fatalf("did not expect at-threshold file to be skipped")
	}

	overMB := make([]byte, 1024*1024+1)
	if err := os.WriteFile(logPath, overMB, 0o600); err != nil {
		t.Fatalf("write over-threshold log failed: %v", err)
	}
	over := makeTailer(1)
	resOver, err := over.PollOnce()
	if err != nil {
		t.Fatalf("over-threshold poll failed: %v", err)
	}
	if !resOver.Skipped {
		t.Fatalf("expected over-threshold file to be skipped")
	}
	if resOver.SkipReason == "" {
		t.Fatalf("expected skip reason for over-threshold file")
	}
	if resOver.BytesRead != 0 || resOver.Chunks != 0 {
		t.Fatalf("expected no reads/chunks when skipped, got %+v", resOver)
	}
}
