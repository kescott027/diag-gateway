package admission

import (
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrNoPeerCertificate      = errors.New("no peer certificate provided")
	ErrInvalidClientCertUsage = errors.New("certificate is not valid for client authentication")
	ErrInvalidCertificateTime = errors.New("certificate outside validity period")
	ErrInvalidSourceIdentity  = errors.New("invalid source identity in certificate")
	ErrUnknownSourceIdentity  = errors.New("unknown source identity")
	ErrInactiveSourceIdentity = errors.New("inactive source identity")
	ErrRevokedCertificate     = errors.New("revoked certificate serial")
)

// SourceStatus defines allowed admission states for a source identity.
type SourceStatus string

const (
	SourceStatusUnknown  SourceStatus = "unknown"
	SourceStatusActive   SourceStatus = "active"
	SourceStatusRevoked  SourceStatus = "revoked"
	SourceStatusDisabled SourceStatus = "disabled"
)

// Registry returns source identity status for admission checks.
type Registry interface {
	GetStatus(sourceID string) SourceStatus
}

// RevocationChecker indicates whether a certificate serial has been revoked.
type RevocationChecker interface {
	IsRevoked(serial string) bool
}

// InMemoryRegistry provides deterministic status checks for current source identities.
type InMemoryRegistry struct {
	mu     sync.RWMutex
	status map[string]SourceStatus
}

func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{status: make(map[string]SourceStatus)}
}

func (r *InMemoryRegistry) SetStatus(sourceID string, st SourceStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status[sourceID] = st
}

func (r *InMemoryRegistry) GetStatus(sourceID string) SourceStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if st, ok := r.status[sourceID]; ok {
		return st
	}
	return SourceStatusUnknown
}

// Validator authenticates peer identity and enforces source admission policy.
type Validator struct {
	registry    Registry
	revocations RevocationChecker
	now         func() time.Time
}

func NewValidator(registry Registry) *Validator {
	return &Validator{registry: registry, now: time.Now}
}

func NewValidatorWithRevocation(registry Registry, revocations RevocationChecker) *Validator {
	return &Validator{registry: registry, revocations: revocations, now: time.Now}
}

func (v *Validator) SetNow(now func() time.Time) {
	if now != nil {
		v.now = now
	}
}

// ValidatePeerCertificate validates cert posture and source activity status.
func (v *Validator) ValidatePeerCertificate(cert *x509.Certificate) (string, error) {
	if cert == nil {
		return "", ErrNoPeerCertificate
	}
	if !hasClientAuthUsage(cert) {
		return "", ErrInvalidClientCertUsage
	}

	now := v.now().UTC()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return "", ErrInvalidCertificateTime
	}
	if v.revocations != nil && cert.SerialNumber != nil && v.revocations.IsRevoked(cert.SerialNumber.String()) {
		return "", ErrRevokedCertificate
	}

	sourceID, err := extractSourceID(cert)
	if err != nil {
		return "", err
	}

	status := v.registry.GetStatus(sourceID)
	switch status {
	case SourceStatusActive:
		return sourceID, nil
	case SourceStatusRevoked, SourceStatusDisabled:
		return "", fmt.Errorf("%w: %s", ErrInactiveSourceIdentity, sourceID)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownSourceIdentity, sourceID)
	}
}

func hasClientAuthUsage(cert *x509.Certificate) bool {
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			return true
		}
	}
	return false
}

func extractSourceID(cert *x509.Certificate) (string, error) {
	candidate := strings.TrimSpace(cert.Subject.CommonName)
	if candidate == "" && len(cert.DNSNames) > 0 {
		candidate = strings.TrimSpace(cert.DNSNames[0])
	}
	if candidate == "" {
		return "", ErrInvalidSourceIdentity
	}
	if strings.Contains(candidate, "/") || strings.Contains(candidate, "\\") || strings.Contains(candidate, " ") {
		return "", ErrInvalidSourceIdentity
	}
	return candidate, nil
}
