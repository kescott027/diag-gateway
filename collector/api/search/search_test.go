package search

import (
	"testing"

	"github.com/kescott027/diag-gateway/collector/stream/reassembly"
)

func TestSearchSubstringAndRegex(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-a", "app.log"); err != nil {
		t.Fatalf("open stream-a failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-a", 0, []byte("Error one\ninfo two\nERROR three\n")); err != nil {
		t.Fatalf("append stream-a failed: %v", err)
	}
	if err := r.CloseStream("source-1", "stream-a", "completed"); err != nil {
		t.Fatalf("close stream-a failed: %v", err)
	}

	svc := NewService(tmp)
	res, err := svc.SearchSource("source-1", Query{
		Pattern:       "error",
		UseRegex:      false,
		CaseSensitive: false,
		MaxResults:    10,
	})
	if err != nil {
		t.Fatalf("substring search failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 substring hits, got %d", len(res))
	}
	if res[0].LineNumber != 1 || res[1].LineNumber != 3 {
		t.Fatalf("unexpected line numbers: %+v", res)
	}

	resRegex, err := svc.SearchSource("source-1", Query{
		Pattern:       "Error\\s+one|ERROR\\s+three",
		UseRegex:      true,
		CaseSensitive: true,
		MaxResults:    10,
	})
	if err != nil {
		t.Fatalf("regex search failed: %v", err)
	}
	if len(resRegex) != 2 {
		t.Fatalf("expected 2 regex hits, got %d", len(resRegex))
	}
}

func TestSearchInvalidRegex(t *testing.T) {
	svc := NewService(t.TempDir())
	if _, err := svc.Search(Query{
		Pattern:       "[bad",
		UseRegex:      true,
		CaseSensitive: true,
	}); err == nil {
		t.Fatalf("expected invalid regex error")
	}
}

func TestSearchResultLimit(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-a", "app.log"); err != nil {
		t.Fatalf("open stream failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-a", 0, []byte("x\nx\nx\nx\n")); err != nil {
		t.Fatalf("append stream failed: %v", err)
	}
	if err := r.CloseStream("source-1", "stream-a", "completed"); err != nil {
		t.Fatalf("close stream failed: %v", err)
	}

	svc := NewService(tmp)
	res, err := svc.Search(Query{
		Pattern:    "x",
		UseRegex:   false,
		MaxResults: 2,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results due limit, got %d", len(res))
	}
}

func TestSearchEmptyCatalog(t *testing.T) {
	svc := NewService(t.TempDir())
	res, err := svc.Search(Query{
		Pattern:    "anything",
		UseRegex:   false,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("search empty catalog failed: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty results, got %+v", res)
	}
}

