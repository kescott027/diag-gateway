package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cursor tracks a persisted file read position.
type Cursor struct {
	SourceID     string `json:"source_id"`
	FileKey      string `json:"file_key"`
	FileIdentity string `json:"file_identity"`
	Offset       int64  `json:"offset"`
	UpdatedAt    string `json:"updated_at"`
}

// Store persists cursor state to disk for restart-safe recovery.
type Store struct {
	path string
	now  func() time.Time

	mu      sync.Mutex
	cursors map[string]Cursor
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		path = "./agent-data/cursors.json"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create cursor dir: %w", err)
	}
	s := &Store{path: path, now: time.Now, cursors: make(map[string]Cursor)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Set(sourceID, fileKey, fileIdentity string, offset int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := compositeKey(sourceID, fileKey)
	s.cursors[key] = Cursor{
		SourceID:     sourceID,
		FileKey:      fileKey,
		FileIdentity: fileIdentity,
		Offset:       offset,
		UpdatedAt:    s.now().UTC().Format(time.RFC3339),
	}
	return s.persist()
}

func (s *Store) Get(sourceID, fileKey string) (Cursor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.cursors[compositeKey(sourceID, fileKey)]
	return cur, ok
}

func (s *Store) Delete(sourceID, fileKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cursors, compositeKey(sourceID, fileKey))
	return s.persist()
}

func (s *Store) All() []Cursor {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Cursor, 0, len(s.cursors))
	for _, cur := range s.cursors {
		out = append(out, cur)
	}
	return out
}

func (s *Store) load() error {
	blob, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read cursor file: %w", err)
	}
	if len(blob) == 0 {
		return nil
	}

	var rows []Cursor
	if err := json.Unmarshal(blob, &rows); err != nil {
		backup := fmt.Sprintf("%s.corrupt-%d", s.path, time.Now().Unix())
		if renameErr := os.Rename(s.path, backup); renameErr != nil {
			return fmt.Errorf("cursor file corrupt (%v) and failed backup rename (%v)", err, renameErr)
		}
		s.cursors = make(map[string]Cursor)
		return nil
	}

	for _, row := range rows {
		s.cursors[compositeKey(row.SourceID, row.FileKey)] = row
	}
	return nil
}

func (s *Store) persist() error {
	rows := make([]Cursor, 0, len(s.cursors))
	for _, cur := range s.cursors {
		rows = append(rows, cur)
	}
	blob, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cursors: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return fmt.Errorf("write cursor tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("promote cursor file: %w", err)
	}
	return nil
}

func compositeKey(sourceID, fileKey string) string {
	return sourceID + "|" + fileKey
}
