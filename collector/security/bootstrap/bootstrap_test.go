package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureLocalPKICreatesArtifacts(t *testing.T) {
	tmp := t.TempDir()
	paths, err := EnsureLocalPKI(Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	checkExists(t, paths.CAKeyPath)
	checkExists(t, paths.CACertPath)
	checkExists(t, paths.ServerKeyPath)
	checkExists(t, paths.ServerCertPath)

	if filepath.Dir(paths.CAKeyPath) != filepath.Join(tmp, "ca") {
		t.Fatalf("unexpected CA dir: %s", filepath.Dir(paths.CAKeyPath))
	}
	if filepath.Dir(paths.ServerKeyPath) != filepath.Join(tmp, "certs") {
		t.Fatalf("unexpected cert dir: %s", filepath.Dir(paths.ServerKeyPath))
	}
}

func TestEnsureLocalPKIIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	paths, err := EnsureLocalPKI(Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	orig, err := os.ReadFile(paths.CACertPath)
	if err != nil {
		t.Fatalf("read original CA cert: %v", err)
	}

	if _, err := EnsureLocalPKI(Config{OutputDir: tmp}); err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	after, err := os.ReadFile(paths.CACertPath)
	if err != nil {
		t.Fatalf("read CA cert after second run: %v", err)
	}

	if string(orig) != string(after) {
		t.Fatalf("CA cert changed across idempotent bootstrap")
	}
}

func checkExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}
