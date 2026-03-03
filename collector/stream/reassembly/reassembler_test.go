package reassembly

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

func TestAppendOnlyReassembly(t *testing.T) {
	tmp := t.TempDir()
	r := NewReassembler(tmp)
	r.now = func() time.Time { return time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC) }

	if err := r.OpenStream("source-1", "stream-1", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}

	next, err := r.AppendChunk("source-1", "stream-1", 0, []byte("hello"))
	if err != nil {
		t.Fatalf("append chunk failed: %v", err)
	}
	if next != 5 {
		t.Fatalf("expected next offset 5, got %d", next)
	}

	next, err = r.AppendChunk("source-1", "stream-1", 5, []byte(" world"))
	if err != nil {
		t.Fatalf("append second chunk failed: %v", err)
	}
	if next != 11 {
		t.Fatalf("expected next offset 11, got %d", next)
	}

	_, err = r.AppendChunk("source-1", "stream-1", 3, []byte("bad"))
	if !errors.Is(err, ErrOffsetMismatch) {
		t.Fatalf("expected offset mismatch, got: %v", err)
	}

	logPath, err := layout.StreamLogPath(tmp, "source-1", "stream-1")
	if err != nil {
		t.Fatalf("stream path failed: %v", err)
	}
	blob, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read stream failed: %v", err)
	}
	if string(blob) != "hello world" {
		t.Fatalf("unexpected stream contents: %q", string(blob))
	}

	if err := r.CloseStream("source-1", "stream-1", "completed"); err != nil {
		t.Fatalf("close stream failed: %v", err)
	}

	metaPath, err := layout.StreamMetadataPath(tmp, "source-1", "stream-1")
	if err != nil {
		t.Fatalf("stream metadata path failed: %v", err)
	}
	metaBlob, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata failed: %v", err)
	}
	var meta Metadata
	if err := json.Unmarshal(metaBlob, &meta); err != nil {
		t.Fatalf("unmarshal metadata failed: %v", err)
	}
	if meta.Status != "closed" || meta.LastOffset != 11 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
}

func TestReassemblerResumesFromExistingOffset(t *testing.T) {
	tmp := t.TempDir()
	r1 := NewReassembler(tmp)
	if err := r1.OpenStream("source-1", "stream-2", "service.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}
	if _, err := r1.AppendChunk("source-1", "stream-2", 0, []byte("12345")); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	// New reassembler instance should continue from on-disk size.
	r2 := NewReassembler(tmp)
	next, err := r2.AppendChunk("source-1", "stream-2", 5, []byte("6789"))
	if err != nil {
		t.Fatalf("append after restart failed: %v", err)
	}
	if next != 9 {
		t.Fatalf("expected offset 9, got %d", next)
	}
}
