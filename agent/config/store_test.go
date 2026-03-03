package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreReplaceGetReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	now := time.Date(2026, 3, 3, 13, 12, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	cfg := AgentConfig{
		SourceID:     "source-1",
		CollectorURL: "https://collector.local:8443",
		Watch: []WatchRule{
			{
				Path:           "./logs/app",
				Include:        []string{"*.log"},
				Exclude:        []string{"*.gz"},
				SendLastNLines: 100,
				MaxFileSizeMB:  200,
			},
		},
	}
	if err := store.Replace(cfg); err != nil {
		t.Fatalf("replace config failed: %v", err)
	}

	out := store.Get()
	if len(out.Watch) != 1 {
		t.Fatalf("expected one watch rule, got %d", len(out.Watch))
	}
	if out.UpdatedAt == "" {
		t.Fatalf("expected updated_at timestamp")
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload store failed: %v", err)
	}
	reloadedCfg := reloaded.Get()
	if reloadedCfg.CollectorURL != "https://collector.local:8443" {
		t.Fatalf("unexpected collector url after reload: %s", reloadedCfg.CollectorURL)
	}
	if len(reloadedCfg.Watch) != 1 {
		t.Fatalf("expected one watch rule after reload, got %d", len(reloadedCfg.Watch))
	}
}

func TestStoreValidationFailures(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	if err := store.Replace(AgentConfig{
		CollectorURL: "http://collector.local:8443",
	}); err == nil {
		t.Fatalf("expected non-https collector_url validation error")
	}

	if err := store.Replace(AgentConfig{
		CollectorURL: "https://collector.local:8443",
		Watch: []WatchRule{
			{Path: "/tmp/a", MaxFileSizeMB: 0},
		},
	}); err == nil {
		t.Fatalf("expected max_file_size_mb validation error")
	}

	if err := store.Replace(AgentConfig{
		CollectorURL: "https://collector.local:8443",
		Watch: []WatchRule{
			{Path: "/tmp/a", Include: []string{"["}, MaxFileSizeMB: 1},
		},
	}); err == nil {
		t.Fatalf("expected include pattern validation error")
	}
}

func TestStoreAddAndRemoveWatchPath(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	if err := store.Replace(AgentConfig{
		CollectorURL: "https://collector.local:8443",
	}); err != nil {
		t.Fatalf("replace config failed: %v", err)
	}

	if err := store.AddWatchRule(WatchRule{
		Path:          "./logs/service",
		Include:       []string{"*.log"},
		MaxFileSizeMB: 100,
	}); err != nil {
		t.Fatalf("add watch rule failed: %v", err)
	}
	if err := store.AddWatchRule(WatchRule{
		Path:          "./logs/service",
		Include:       []string{"*.txt"},
		MaxFileSizeMB: 100,
	}); err == nil {
		t.Fatalf("expected duplicate watch path validation error")
	}

	cfg := store.Get()
	if len(cfg.Watch) != 1 {
		t.Fatalf("expected one watch rule, got %d", len(cfg.Watch))
	}

	if err := store.RemoveWatchPath("./logs/service"); err != nil {
		t.Fatalf("remove watch path failed: %v", err)
	}
	cfg = store.Get()
	if len(cfg.Watch) != 0 {
		t.Fatalf("expected zero watch rules after removal, got %d", len(cfg.Watch))
	}
}

func TestStoreCorruptionFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatalf("write corrupt config failed: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store with corrupt config failed: %v", err)
	}
	if len(store.Get().Watch) != 0 {
		t.Fatalf("expected empty watch config after corruption fallback")
	}

	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob corrupt backup failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one corrupt backup file, got %d", len(matches))
	}
}

