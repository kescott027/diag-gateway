package fileid

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeFileInfo struct {
	name string
	size int64
	mod  time.Time
	sys  any
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return f.mod }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return f.sys }

func TestIdentifyPathProducesIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	id, err := IdentifyPath(path)
	if err != nil {
		t.Fatalf("identify path failed: %v", err)
	}
	if id.Path == "" {
		t.Fatalf("expected normalized path")
	}
	if id.Method == MethodNativeInodeDev {
		if id.Confidence != ConfidenceStrong {
			t.Fatalf("expected strong confidence for native identity")
		}
		if id.Inode == 0 {
			t.Fatalf("expected non-zero inode for native identity")
		}
	} else if id.Method == MethodPathSizeMTime {
		if id.Confidence != ConfidenceFallback {
			t.Fatalf("expected fallback confidence")
		}
	} else {
		t.Fatalf("unexpected method: %s", id.Method)
	}
}

func TestIdentifyFallsBackWithoutNativeSysInfo(t *testing.T) {
	mod := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	info := fakeFileInfo{name: "x.log", size: 42, mod: mod}

	id, err := Identify("relative/x.log", info)
	if err != nil {
		t.Fatalf("identify failed: %v", err)
	}
	if id.Method != MethodPathSizeMTime {
		t.Fatalf("expected fallback method, got %s", id.Method)
	}
	if id.Confidence != ConfidenceFallback {
		t.Fatalf("expected fallback confidence, got %s", id.Confidence)
	}
	if id.Size != 42 {
		t.Fatalf("unexpected size: %d", id.Size)
	}
}

func TestSameEntityFallbackIgnoresAppendMutations(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "service.log")
	a := Identity{
		Version:         Version,
		Method:          MethodPathSizeMTime,
		Confidence:      ConfidenceFallback,
		Path:            basePath,
		Size:            100,
		ModTimeUnixNano: 1000,
	}
	b := Identity{
		Version:         Version,
		Method:          MethodPathSizeMTime,
		Confidence:      ConfidenceFallback,
		Path:            basePath,
		Size:            200,
		ModTimeUnixNano: 2000,
	}

	if !a.SameEntity(b) {
		t.Fatalf("expected fallback identities with same path to match")
	}
}

func TestEncodeParseRoundTrip(t *testing.T) {
	in := Identity{
		Version:         Version,
		Method:          MethodNativeInodeDev,
		Confidence:      ConfidenceStrong,
		Path:            "/var/log/app.log",
		Device:          12,
		Inode:           34,
		Size:            56,
		ModTimeUnixNano: 78,
	}
	raw := in.Encode()
	out, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !in.SameEntity(out) {
		t.Fatalf("expected same entity after parse")
	}
	if out.Size != 56 {
		t.Fatalf("unexpected size: %d", out.Size)
	}
}

func TestParseLegacyInodeToken(t *testing.T) {
	out, err := Parse("inode:1234")
	if err != nil {
		t.Fatalf("parse legacy failed: %v", err)
	}
	if out.Method != MethodNativeInodeDev {
		t.Fatalf("unexpected method: %s", out.Method)
	}
	if out.Inode != 1234 {
		t.Fatalf("unexpected inode: %d", out.Inode)
	}
}

