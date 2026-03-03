package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// WatchRule defines one watched path and its collection constraints.
type WatchRule struct {
	Path          string   `json:"path"`
	Include       []string `json:"include,omitempty"`
	Exclude       []string `json:"exclude,omitempty"`
	SendLastNLines int     `json:"send_last_n_lines,omitempty"`
	MaxFileSizeMB int      `json:"max_file_size_mb"`
}

// AgentConfig is the persisted local configuration model for agent UI operations.
type AgentConfig struct {
	SourceID     string      `json:"source_id,omitempty"`
	CollectorURL string      `json:"collector_url,omitempty"`
	Watch        []WatchRule `json:"watch"`
	UpdatedAt    string      `json:"updated_at"`
}

// Store persists agent configuration atomically with validation.
type Store struct {
	path string
	now  func() time.Time

	mu  sync.Mutex
	cfg AgentConfig
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		path = "./agent-data/config.json"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	s := &Store{
		path: path,
		now:  time.Now,
		cfg:  AgentConfig{Watch: []WatchRule{}},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Get() AgentConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return copyConfig(s.cfg)
}

func (s *Store) Replace(cfg AgentConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replaceLocked(cfg)
}

func (s *Store) AddWatchRule(rule WatchRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := copyConfig(s.cfg)
	cfg.Watch = append(cfg.Watch, rule)
	return s.replaceLocked(cfg)
}

func (s *Store) RemoveWatchPath(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedPath, err := normalizePath(path)
	if err != nil {
		return err
	}

	cfg := copyConfig(s.cfg)
	filtered := make([]WatchRule, 0, len(cfg.Watch))
	for _, rule := range cfg.Watch {
		if normalizeRulePathUnsafe(rule.Path) == normalizedPath {
			continue
		}
		filtered = append(filtered, rule)
	}
	cfg.Watch = filtered
	return s.replaceLocked(cfg)
}

func (s *Store) replaceLocked(cfg AgentConfig) error {
	normalized, err := validateAndNormalize(cfg)
	if err != nil {
		return err
	}
	normalized.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.persistLocked(normalized); err != nil {
		return err
	}
	s.cfg = normalized
	return nil
}

func validateAndNormalize(cfg AgentConfig) (AgentConfig, error) {
	out := AgentConfig{
		SourceID:     strings.TrimSpace(cfg.SourceID),
		CollectorURL: strings.TrimSpace(cfg.CollectorURL),
		Watch:        make([]WatchRule, 0, len(cfg.Watch)),
	}

	if out.CollectorURL != "" {
		u, err := url.Parse(out.CollectorURL)
		if err != nil {
			return AgentConfig{}, fmt.Errorf("collector_url parse failed: %w", err)
		}
		if strings.ToLower(u.Scheme) != "https" {
			return AgentConfig{}, fmt.Errorf("collector_url must use https")
		}
	}

	seen := make(map[string]struct{})
	for _, rule := range cfg.Watch {
		path, err := normalizePath(rule.Path)
		if err != nil {
			return AgentConfig{}, err
		}
		if _, ok := seen[path]; ok {
			return AgentConfig{}, fmt.Errorf("duplicate watch path: %s", path)
		}
		seen[path] = struct{}{}

		if rule.MaxFileSizeMB <= 0 {
			return AgentConfig{}, fmt.Errorf("max_file_size_mb must be > 0 for %s", path)
		}
		if rule.SendLastNLines < 0 {
			return AgentConfig{}, fmt.Errorf("send_last_n_lines must be >= 0 for %s", path)
		}
		if err := validatePatterns(rule.Include); err != nil {
			return AgentConfig{}, fmt.Errorf("include patterns for %s: %w", path, err)
		}
		if err := validatePatterns(rule.Exclude); err != nil {
			return AgentConfig{}, fmt.Errorf("exclude patterns for %s: %w", path, err)
		}

		out.Watch = append(out.Watch, WatchRule{
			Path:           path,
			Include:        append([]string(nil), rule.Include...),
			Exclude:        append([]string(nil), rule.Exclude...),
			SendLastNLines: rule.SendLastNLines,
			MaxFileSizeMB:  rule.MaxFileSizeMB,
		})
	}

	sort.Slice(out.Watch, func(i, j int) bool { return out.Watch[i].Path < out.Watch[j].Path })
	return out, nil
}

func validatePatterns(patterns []string) error {
	for _, pattern := range patterns {
		p := strings.TrimSpace(pattern)
		if p == "" {
			return fmt.Errorf("pattern must not be empty")
		}
		if _, err := filepath.Match(p, "sample.log"); err != nil {
			return fmt.Errorf("invalid glob pattern %q: %w", p, err)
		}
	}
	return nil
}

func normalizePath(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	clean := filepath.Clean(p)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("normalize path %q: %w", p, err)
	}
	return abs, nil
}

func normalizeRulePathUnsafe(path string) string {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

func (s *Store) load() error {
	blob, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}
	if len(blob) == 0 {
		return nil
	}

	var cfg AgentConfig
	if err := json.Unmarshal(blob, &cfg); err != nil {
		backup := fmt.Sprintf("%s.corrupt-%d", s.path, time.Now().Unix())
		if renameErr := os.Rename(s.path, backup); renameErr != nil {
			return fmt.Errorf("config file corrupt (%v) and backup rename failed (%v)", err, renameErr)
		}
		s.cfg = AgentConfig{Watch: []WatchRule{}}
		return nil
	}

	normalized, err := validateAndNormalize(cfg)
	if err != nil {
		return fmt.Errorf("validate loaded config: %w", err)
	}
	s.cfg = normalized
	return nil
}

func (s *Store) persistLocked(cfg AgentConfig) error {
	blob, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return fmt.Errorf("write config temp file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("promote config file: %w", err)
	}
	return nil
}

func copyConfig(in AgentConfig) AgentConfig {
	out := AgentConfig{
		SourceID:     in.SourceID,
		CollectorURL: in.CollectorURL,
		UpdatedAt:    in.UpdatedAt,
		Watch:        make([]WatchRule, 0, len(in.Watch)),
	}
	for _, rule := range in.Watch {
		out.Watch = append(out.Watch, WatchRule{
			Path:           rule.Path,
			Include:        append([]string(nil), rule.Include...),
			Exclude:        append([]string(nil), rule.Exclude...),
			SendLastNLines: rule.SendLastNLines,
			MaxFileSizeMB:  rule.MaxFileSizeMB,
		})
	}
	return out
}

