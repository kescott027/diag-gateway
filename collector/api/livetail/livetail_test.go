package livetail

import (
	"errors"
	"testing"

	"github.com/kescott027/diag-gateway/collector/stream/reassembly"
)

func TestPollReadsIncrementalUpdates(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-a", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-a", 0, []byte("one\ntwo\n")); err != nil {
		t.Fatalf("append initial chunk failed: %v", err)
	}

	svc := NewService(tmp)
	first, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: 0, MaxBytes: 1024, MaxLines: 10})
	if err != nil {
		t.Fatalf("first poll failed: %v", err)
	}
	if len(first.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(first.Entries))
	}
	if !first.EOF {
		t.Fatalf("expected eof true after reading all data")
	}

	if _, err := r.AppendChunk("source-1", "stream-a", first.NextOffset, []byte("three\n")); err != nil {
		t.Fatalf("append second chunk failed: %v", err)
	}

	second, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: first.NextOffset, MaxBytes: 1024, MaxLines: 10})
	if err != nil {
		t.Fatalf("second poll failed: %v", err)
	}
	if len(second.Entries) != 1 || second.Entries[0].Line != "three" {
		t.Fatalf("unexpected second poll entries: %+v", second.Entries)
	}
	if second.Entries[0].Offset != first.NextOffset {
		t.Fatalf("expected entry offset %d, got %d", first.NextOffset, second.Entries[0].Offset)
	}
}

func TestPollMaxLinesBound(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-a", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-a", 0, []byte("a\nb\nc\n")); err != nil {
		t.Fatalf("append chunk failed: %v", err)
	}

	svc := NewService(tmp)
	res, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: 0, MaxBytes: 1024, MaxLines: 2})
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(res.Entries))
	}
	if !res.Truncated {
		t.Fatalf("expected truncated response when max_lines is reached")
	}
	if res.NextOffset <= 0 {
		t.Fatalf("expected advanced next offset, got %d", res.NextOffset)
	}

	followUp, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: res.NextOffset, MaxBytes: 1024, MaxLines: 2})
	if err != nil {
		t.Fatalf("follow-up poll failed: %v", err)
	}
	if len(followUp.Entries) != 1 || followUp.Entries[0].Line != "c" {
		t.Fatalf("unexpected follow-up entries: %+v", followUp.Entries)
	}
}

func TestPollPartialLine(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-a", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-a", 0, []byte("partial")); err != nil {
		t.Fatalf("append chunk failed: %v", err)
	}

	svc := NewService(tmp)
	res, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: 0, MaxBytes: 1024, MaxLines: 10})
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if len(res.Entries) != 1 || !res.Entries[0].Partial {
		t.Fatalf("expected one partial entry, got %+v", res.Entries)
	}
	if res.Entries[0].Line != "partial" {
		t.Fatalf("unexpected partial line payload: %q", res.Entries[0].Line)
	}
}

func TestPollRejectsInvalidAndMissingStreams(t *testing.T) {
	svc := NewService(t.TempDir())
	if _, err := svc.Poll(Query{SourceID: "bad/source", StreamID: "stream-a", Offset: 0}); err == nil {
		t.Fatalf("expected invalid id error")
	}

	_, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: 0})
	if !errors.Is(err, ErrStreamNotFound) {
		t.Fatalf("expected ErrStreamNotFound, got %v", err)
	}
}

func TestPollNormalizesOffsetAndLimits(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-a", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-a", 0, []byte("a\n")); err != nil {
		t.Fatalf("append chunk failed: %v", err)
	}

	svc := NewService(tmp)
	res, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: 9999, MaxBytes: 99999999, MaxLines: 99999999})
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.Offset != res.NextOffset {
		t.Fatalf("expected clamped offset at eof; got offset=%d next=%d", res.Offset, res.NextOffset)
	}
	if !res.EOF {
		t.Fatalf("expected eof true at clamped offset")
	}
}

func TestPollMaxBytesSetsTruncated(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-a", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-a", 0, []byte("aaaa\nbbbb\n")); err != nil {
		t.Fatalf("append chunk failed: %v", err)
	}

	svc := NewService(tmp)
	res, err := svc.Poll(Query{SourceID: "source-1", StreamID: "stream-a", Offset: 0, MaxBytes: 5, MaxLines: 100})
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated=true when max_bytes bound is reached")
	}
	if res.NextOffset != 5 {
		t.Fatalf("expected next offset 5, got %d", res.NextOffset)
	}
}
