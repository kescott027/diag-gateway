package configpush

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/audittrail"
)

var (
	ErrInvalidSourceID     = errors.New("invalid source id")
	ErrInvalidConfigHash   = errors.New("invalid config hash")
	ErrUpdateNotFound      = errors.New("config update not found")
	ErrInvalidTransition   = errors.New("invalid update status transition")
	ErrFailureReasonNeeded = errors.New("failure reason is required")
)

var sourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Status defines config update lifecycle state.
type Status string

const (
	StatusPending Status = "pending"
	StatusApplied Status = "applied"
	StatusFailed  Status = "failed"
)

// Update captures one remote-config update lifecycle record.
type Update struct {
	ID            string    `json:"id"`
	SourceID      string    `json:"source_id"`
	ConfigHash    string    `json:"config_hash"`
	SubmittedBy   string    `json:"submitted_by"`
	SubmittedAt   time.Time `json:"submitted_at"`
	Status        Status    `json:"status"`
	AppliedAt     time.Time `json:"applied_at,omitempty"`
	FailedAt      time.Time `json:"failed_at,omitempty"`
	FailedBy      string    `json:"failed_by,omitempty"`
	FailureReason string    `json:"failure_reason,omitempty"`
}

// SubmitRequest carries config-update intent parameters.
type SubmitRequest struct {
	SourceID    string `json:"source_id"`
	ConfigHash  string `json:"config_hash"`
	SubmittedBy string `json:"submitted_by"`
}

// Filter scopes list queries.
type Filter struct {
	SourceID string  `json:"source_id,omitempty"`
	Status   *Status `json:"status,omitempty"`
	Limit    int     `json:"limit,omitempty"`
}

type auditAppender interface {
	Append(event audittrail.Event) error
}

// Service stores config update intents with deterministic lifecycle transitions.
type Service struct {
	maxUpdates int
	audit      auditAppender
	now        func() time.Time

	mu       sync.Mutex
	updates  map[string]Update
	order    []string
	sequence uint64
}

func NewService(maxUpdates int, audit auditAppender) *Service {
	if maxUpdates <= 0 {
		maxUpdates = 1000
	}
	return &Service{
		maxUpdates: maxUpdates,
		audit:      audit,
		now:        time.Now,
		updates:    make(map[string]Update),
		order:      make([]string, 0, maxUpdates),
	}
}

// Submit creates a pending update and emits an audit event.
func (s *Service) Submit(req SubmitRequest) (Update, error) {
	sourceID := strings.TrimSpace(req.SourceID)
	if !sourceIDPattern.MatchString(sourceID) {
		return Update{}, ErrInvalidSourceID
	}
	configHash := normalizeHash(req.ConfigHash)
	if configHash == "" {
		return Update{}, ErrInvalidConfigHash
	}
	submittedBy := strings.TrimSpace(req.SubmittedBy)
	submittedAt := s.now().UTC()

	s.mu.Lock()
	s.sequence++
	id := buildID(sourceID, configHash, submittedAt, s.sequence)
	update := Update{
		ID:          id,
		SourceID:    sourceID,
		ConfigHash:  configHash,
		SubmittedBy: submittedBy,
		SubmittedAt: submittedAt,
		Status:      StatusPending,
	}
	s.updates[id] = update
	s.order = append(s.order, id)
	s.evictLocked()
	s.mu.Unlock()

	s.auditEvent("config.submit", submittedBy, sourceID, id, "", "")
	return update, nil
}

// MarkApplied transitions a pending update to applied.
func (s *Service) MarkApplied(id, actor string) (Update, error) {
	actor = strings.TrimSpace(actor)
	s.mu.Lock()
	update, ok := s.updates[id]
	if !ok {
		s.mu.Unlock()
		return Update{}, ErrUpdateNotFound
	}
	if update.Status != StatusPending {
		s.mu.Unlock()
		return Update{}, ErrInvalidTransition
	}
	update.Status = StatusApplied
	update.AppliedAt = s.now().UTC()
	s.updates[id] = update
	s.mu.Unlock()

	s.auditEvent("config.apply", actor, update.SourceID, update.ID, "", "")
	return update, nil
}

// MarkFailed transitions a pending update to failed with reason.
func (s *Service) MarkFailed(id, actor, reason string) (Update, error) {
	actor = strings.TrimSpace(actor)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Update{}, ErrFailureReasonNeeded
	}

	s.mu.Lock()
	update, ok := s.updates[id]
	if !ok {
		s.mu.Unlock()
		return Update{}, ErrUpdateNotFound
	}
	if update.Status != StatusPending {
		s.mu.Unlock()
		return Update{}, ErrInvalidTransition
	}
	update.Status = StatusFailed
	update.FailedAt = s.now().UTC()
	update.FailedBy = actor
	update.FailureReason = reason
	s.updates[id] = update
	s.mu.Unlock()

	s.auditEvent("config.fail", actor, update.SourceID, update.ID, reason, "")
	return update, nil
}

func (s *Service) Get(id string) (Update, error) {
	s.mu.Lock()
	update, ok := s.updates[id]
	s.mu.Unlock()
	if !ok {
		return Update{}, ErrUpdateNotFound
	}
	return update, nil
}

func (s *Service) List(filter Filter) []Update {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	sourceID := strings.TrimSpace(filter.SourceID)
	s.mu.Lock()
	out := make([]Update, 0, limit)
	for _, id := range s.order {
		update, ok := s.updates[id]
		if !ok {
			continue
		}
		if sourceID != "" && update.SourceID != sourceID {
			continue
		}
		if filter.Status != nil && update.Status != *filter.Status {
			continue
		}
		out = append(out, update)
		if len(out) >= limit {
			break
		}
	}
	s.mu.Unlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SubmittedAt.Equal(out[j].SubmittedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].SubmittedAt.Before(out[j].SubmittedAt)
	})
	return out
}

func (s *Service) evictLocked() {
	if len(s.order) <= s.maxUpdates {
		return
	}
	trim := len(s.order) - s.maxUpdates
	for i := 0; i < trim; i++ {
		delete(s.updates, s.order[i])
	}
	s.order = append([]string(nil), s.order[trim:]...)
}

func (s *Service) auditEvent(action, actor, sourceID, updateID, reason, outcome string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Append(audittrail.Event{
		Timestamp: s.now().UTC(),
		Actor:     actor,
		Action:    action,
		SourceID:  sourceID,
		Outcome:   outcome,
		Reason:    reason,
		Metadata:  map[string]any{"update_id": updateID},
	})
}

func normalizeHash(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if strings.TrimPrefix(trimmed, "sha256:") == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "sha256:")
	if len(trimmed) == 64 && isHex(trimmed) {
		return "sha256:" + trimmed
	}
	return ""
}

func isHex(value string) bool {
	for _, ch := range value {
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') {
			continue
		}
		return false
	}
	return true
}

func buildID(sourceID, configHash string, ts time.Time, sequence uint64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%d", sourceID, configHash, ts.UnixNano(), sequence)))
	return "cfg_" + hex.EncodeToString(sum[:8])
}
