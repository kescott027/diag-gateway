package revocation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/admission"
)

// SourceRegistry defines source status mutation and lookup.
type SourceRegistry interface {
	SetStatus(sourceID string, status admission.SourceStatus)
	GetStatus(sourceID string) admission.SourceStatus
}

// SerialRevoker marks certificate serials as revoked at specific timestamps.
type SerialRevoker interface {
	RevokeAfter(serial string, at time.Time)
}

// AuditEvent captures immutable revocation-related audit records.
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	SourceID  string    `json:"source_id,omitempty"`
	Serial    string    `json:"serial,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// AuditSink accepts append-only audit events.
type AuditSink interface {
	Append(event AuditEvent) error
}

// AppendOnlyFileAuditSink writes newline-delimited JSON events in append-only mode.
type AppendOnlyFileAuditSink struct {
	Path string
	mu   sync.Mutex
}

func NewAppendOnlyFileAuditSink(path string) *AppendOnlyFileAuditSink {
	return &AppendOnlyFileAuditSink{Path: path}
}

func (s *AppendOnlyFileAuditSink) Append(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("ensure audit dir: %w", err)
	}

	file, err := os.OpenFile(s.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	blob, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	if _, err := file.Write(append(blob, '\n')); err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	return nil
}

// Service provides immediate source and serial revocation operations.
type Service struct {
	registry SourceRegistry
	serials  SerialRevoker
	audit    AuditSink
	now      func() time.Time
}

func NewService(registry SourceRegistry, serials SerialRevoker, audit AuditSink) *Service {
	return &Service{
		registry: registry,
		serials:  serials,
		audit:    audit,
		now:      time.Now,
	}
}

func (s *Service) RevokeSource(sourceID, actor, reason string) error {
	s.registry.SetStatus(sourceID, admission.SourceStatusRevoked)
	if s.audit != nil {
		return s.audit.Append(AuditEvent{
			Timestamp: s.now().UTC(),
			Actor:     actor,
			Action:    "source_revoked",
			SourceID:  sourceID,
			Reason:    reason,
		})
	}
	return nil
}

func (s *Service) RevokeSerial(serial, actor, reason string) error {
	s.serials.RevokeAfter(serial, s.now().UTC())
	if s.audit != nil {
		return s.audit.Append(AuditEvent{
			Timestamp: s.now().UTC(),
			Actor:     actor,
			Action:    "serial_revoked",
			Serial:    serial,
			Reason:    reason,
		})
	}
	return nil
}
