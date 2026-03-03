package reassembly

import (
	"errors"
	"os"
	"testing"

	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

func TestChunkProcessorInOrderAndDuplicate(t *testing.T) {
	tmp := t.TempDir()
	r := NewReassembler(tmp)
	if err := r.OpenStream("source-1", "stream-seq", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}

	p := NewChunkProcessor(r, 128)

	res, err := p.ProcessChunk("source-1", "stream-seq", 1, 0, []byte("abc"))
	if err != nil {
		t.Fatalf("process first chunk failed: %v", err)
	}
	if res.Duplicate || res.NextExpectedSequence != 2 {
		t.Fatalf("unexpected first result: %+v", res)
	}

	res, err = p.ProcessChunk("source-1", "stream-seq", 1, 0, []byte("abc"))
	if err != nil {
		t.Fatalf("duplicate chunk should not fail: %v", err)
	}
	if !res.Duplicate || res.NextExpectedSequence != 2 {
		t.Fatalf("unexpected duplicate result: %+v", res)
	}

	logPath, err := layout.StreamLogPath(tmp, "source-1", "stream-seq")
	if err != nil {
		t.Fatalf("stream log path failed: %v", err)
	}
	blob, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read stream failed: %v", err)
	}
	if string(blob) != "abc" {
		t.Fatalf("duplicate write corrupted output: %q", string(blob))
	}
}

func TestChunkProcessorOutOfOrderAndConflict(t *testing.T) {
	tmp := t.TempDir()
	r := NewReassembler(tmp)
	if err := r.OpenStream("source-1", "stream-seq2", "svc.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}

	p := NewChunkProcessor(r, 128)

	_, err := p.ProcessChunk("source-1", "stream-seq2", 2, 0, []byte("abc"))
	if !errors.Is(err, ErrOutOfOrderSequence) {
		t.Fatalf("expected out-of-order error, got: %v", err)
	}

	if _, err := p.ProcessChunk("source-1", "stream-seq2", 1, 0, []byte("abc")); err != nil {
		t.Fatalf("process first chunk failed: %v", err)
	}

	_, err = p.ProcessChunk("source-1", "stream-seq2", 1, 1, []byte("z"))
	if !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("expected sequence conflict error, got: %v", err)
	}
}
