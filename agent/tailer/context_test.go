package tailer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadTailContextLastNLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	ctx, err := ReadTailContext(path, 2, 0)
	if err != nil {
		t.Fatalf("read tail context failed: %v", err)
	}
	if len(ctx.Lines) != 2 || ctx.Lines[0] != "three" || ctx.Lines[1] != "four" {
		t.Fatalf("unexpected lines: %+v", ctx.Lines)
	}
	if ctx.Truncated {
		t.Fatalf("did not expect truncated for full-file read")
	}
}

func TestReadTailContextHandlesNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("one\ntwo"), 0o600); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	ctx, err := ReadTailContext(path, 5, 0)
	if err != nil {
		t.Fatalf("read tail context failed: %v", err)
	}
	if len(ctx.Lines) != 2 || ctx.Lines[1] != "two" {
		t.Fatalf("unexpected lines: %+v", ctx.Lines)
	}
}

func TestReadTailContextBoundedReadDropsPartialPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	content := "prefix-long-line\nalpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	ctx, err := ReadTailContext(path, 3, 14)
	if err != nil {
		t.Fatalf("read tail context failed: %v", err)
	}
	if !ctx.Truncated {
		t.Fatalf("expected truncated for bounded suffix read")
	}
	if len(ctx.Lines) != 2 || ctx.Lines[0] != "beta" || ctx.Lines[1] != "gamma" {
		t.Fatalf("unexpected truncated lines: %+v", ctx.Lines)
	}
}

func TestReadTailContextZeroLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	ctx, err := ReadTailContext(path, 0, 0)
	if err != nil {
		t.Fatalf("read tail context failed: %v", err)
	}
	if len(ctx.Lines) != 0 {
		t.Fatalf("expected empty context lines, got %+v", ctx.Lines)
	}
}

func TestReadTailContextMissingFile(t *testing.T) {
	_, err := ReadTailContext(filepath.Join(t.TempDir(), "missing.log"), 10, 0)
	if err == nil {
		t.Fatalf("expected missing-file error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}
