package listing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kescott027/diag-gateway/collector/stream/reassembly"
)

func TestListSourcesSortedAndValidated(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "sources", "source-b"), 0o700); err != nil {
		t.Fatalf("mkdir source-b failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "sources", "source-a"), 0o700); err != nil {
		t.Fatalf("mkdir source-a failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "sources", "bad source"), 0o700); err != nil {
		t.Fatalf("mkdir invalid source failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sources", "not-a-dir"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write non-dir source entry failed: %v", err)
	}

	svc := NewService(tmp)
	sources, err := svc.ListSources()
	if err != nil {
		t.Fatalf("list sources failed: %v", err)
	}
	if len(sources) != 2 || sources[0] != "source-a" || sources[1] != "source-b" {
		t.Fatalf("unexpected sources: %+v", sources)
	}
}

func TestListSourceFilesSortedByLogicalPath(t *testing.T) {
	tmp := t.TempDir()
	r := reassembly.NewReassembler(tmp)

	if err := r.OpenStream("source-1", "stream-2", "z.log"); err != nil {
		t.Fatalf("open stream-2 failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-2", 0, []byte("hello")); err != nil {
		t.Fatalf("append stream-2 failed: %v", err)
	}
	if err := r.CloseStream("source-1", "stream-2", "completed"); err != nil {
		t.Fatalf("close stream-2 failed: %v", err)
	}

	if err := r.OpenStream("source-1", "stream-1", "a.log"); err != nil {
		t.Fatalf("open stream-1 failed: %v", err)
	}
	if _, err := r.AppendChunk("source-1", "stream-1", 0, []byte("abc")); err != nil {
		t.Fatalf("append stream-1 failed: %v", err)
	}
	if err := r.CloseStream("source-1", "stream-1", "completed"); err != nil {
		t.Fatalf("close stream-1 failed: %v", err)
	}

	svc := NewService(tmp)
	files, err := svc.ListSourceFiles("source-1")
	if err != nil {
		t.Fatalf("list source files failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected two files, got %d", len(files))
	}
	if files[0].LogicalPath != "a.log" || files[1].LogicalPath != "z.log" {
		t.Fatalf("expected logical-path sort order, got %+v", files)
	}
	if files[0].LastOffset != 3 || files[1].LastOffset != 5 {
		t.Fatalf("unexpected offsets: %+v", files)
	}
}

func TestListSourceFilesInvalidSourceID(t *testing.T) {
	svc := NewService(t.TempDir())
	if _, err := svc.ListSourceFiles("bad/source"); err == nil {
		t.Fatalf("expected invalid source id error")
	}
}

func TestListCatalogEmpty(t *testing.T) {
	svc := NewService(t.TempDir())
	catalog, err := svc.ListCatalog()
	if err != nil {
		t.Fatalf("list catalog failed: %v", err)
	}
	if len(catalog.Sources) != 0 {
		t.Fatalf("expected empty catalog, got %+v", catalog)
	}
}

