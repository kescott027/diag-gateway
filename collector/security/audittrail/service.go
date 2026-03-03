package audittrail

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultLimit = 200

// Event is one immutable audit-log record.
type Event struct {
	Timestamp  time.Time      `json:"timestamp"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	SourceID   string         `json:"source_id,omitempty"`
	ConfigPath string         `json:"config_path,omitempty"`
	Outcome    string         `json:"outcome,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Filter scopes query results from append-only audit logs.
type Filter struct {
	Actor        string    `json:"actor,omitempty"`
	ActionPrefix string    `json:"action_prefix,omitempty"`
	SourceID     string    `json:"source_id,omitempty"`
	Since        time.Time `json:"since,omitempty"`
	Until        time.Time `json:"until,omitempty"`
	Limit        int       `json:"limit,omitempty"`
}

// Service provides append-only writes and bounded reads for audit events.
type Service struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
}

func NewService(path string) *Service {
	return &Service{
		path: path,
		now:  time.Now,
	}
}

func (s *Service) Append(event Event) error {
	if strings.TrimSpace(event.Action) == "" {
		return fmt.Errorf("action is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = s.now().UTC()
	}
	event.Actor = strings.TrimSpace(event.Actor)
	event.Action = strings.TrimSpace(event.Action)
	event.SourceID = strings.TrimSpace(event.SourceID)
	event.ConfigPath = strings.TrimSpace(event.ConfigPath)
	event.Outcome = strings.TrimSpace(event.Outcome)
	event.Reason = strings.TrimSpace(event.Reason)

	blob, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("ensure audit dir: %w", err)
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(blob, '\n')); err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	return nil
}

func (s *Service) Query(filter Filter) ([]Event, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	actor := strings.TrimSpace(filter.Actor)
	actionPrefix := strings.TrimSpace(filter.ActionPrefix)
	sourceID := strings.TrimSpace(filter.SourceID)

	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	events := make([]Event, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode audit line: %w", err)
		}
		if actor != "" && event.Actor != actor {
			continue
		}
		if actionPrefix != "" && !strings.HasPrefix(event.Action, actionPrefix) {
			continue
		}
		if sourceID != "" && event.SourceID != sourceID {
			continue
		}
		if !filter.Since.IsZero() && event.Timestamp.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && event.Timestamp.After(filter.Until) {
			continue
		}
		events = append(events, event)
		if len(events) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan audit log: %w", err)
	}
	return events, nil
}

func (s *Service) RecordEnrollment(actor, sourceID, outcome, reason string, metadata map[string]any) error {
	return s.Append(Event{
		Actor:    actor,
		Action:   "enrollment.issue",
		SourceID: sourceID,
		Outcome:  outcome,
		Reason:   reason,
		Metadata: metadata,
	})
}

func (s *Service) RecordConfigChange(actor, configPath, outcome, reason string, metadata map[string]any) error {
	return s.Append(Event{
		Actor:      actor,
		Action:     "config.update",
		ConfigPath: configPath,
		Outcome:    outcome,
		Reason:     reason,
		Metadata:   metadata,
	})
}
