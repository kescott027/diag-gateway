package admission

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/crl"
)

func TestValidatePeerCertificateAcceptsActiveSource(t *testing.T) {
	registry := NewInMemoryRegistry()
	registry.SetStatus("source-1", SourceStatusActive)

	validator := NewValidator(registry)
	now := time.Date(2026, 3, 3, 11, 50, 0, 0, time.UTC)
	validator.now = func() time.Time { return now }

	cert := &x509.Certificate{
		Subject:     pkix.Name{CommonName: "source-1"},
		NotBefore:   now.Add(-1 * time.Minute),
		NotAfter:    now.Add(1 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	sourceID, err := validator.ValidatePeerCertificate(cert)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if sourceID != "source-1" {
		t.Fatalf("unexpected source id: %s", sourceID)
	}
}

func TestValidatePeerCertificateRejectsUnknownOrInactive(t *testing.T) {
	registry := NewInMemoryRegistry()
	registry.SetStatus("source-revoked", SourceStatusRevoked)

	validator := NewValidator(registry)
	now := time.Date(2026, 3, 3, 11, 50, 0, 0, time.UTC)
	validator.now = func() time.Time { return now }

	unknown := &x509.Certificate{
		Subject:     pkix.Name{CommonName: "source-unknown"},
		NotBefore:   now.Add(-1 * time.Minute),
		NotAfter:    now.Add(1 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	_, err := validator.ValidatePeerCertificate(unknown)
	if !errors.Is(err, ErrUnknownSourceIdentity) {
		t.Fatalf("expected unknown-source error, got: %v", err)
	}

	revoked := &x509.Certificate{
		Subject:     pkix.Name{CommonName: "source-revoked"},
		NotBefore:   now.Add(-1 * time.Minute),
		NotAfter:    now.Add(1 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	_, err = validator.ValidatePeerCertificate(revoked)
	if !errors.Is(err, ErrInactiveSourceIdentity) {
		t.Fatalf("expected inactive-source error, got: %v", err)
	}
}

func TestValidatePeerCertificateRejectsInvalidCert(t *testing.T) {
	registry := NewInMemoryRegistry()
	validator := NewValidator(registry)
	now := time.Date(2026, 3, 3, 11, 50, 0, 0, time.UTC)
	validator.now = func() time.Time { return now }

	_, err := validator.ValidatePeerCertificate(nil)
	if !errors.Is(err, ErrNoPeerCertificate) {
		t.Fatalf("expected no-cert error, got: %v", err)
	}

	wrongUsage := &x509.Certificate{
		Subject:     pkix.Name{CommonName: "source-1"},
		NotBefore:   now.Add(-1 * time.Minute),
		NotAfter:    now.Add(1 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	_, err = validator.ValidatePeerCertificate(wrongUsage)
	if !errors.Is(err, ErrInvalidClientCertUsage) {
		t.Fatalf("expected client-auth usage error, got: %v", err)
	}

	expired := &x509.Certificate{
		Subject:     pkix.Name{CommonName: "source-1"},
		NotBefore:   now.Add(-2 * time.Hour),
		NotAfter:    now.Add(-1 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	_, err = validator.ValidatePeerCertificate(expired)
	if !errors.Is(err, ErrInvalidCertificateTime) {
		t.Fatalf("expected cert-time error, got: %v", err)
	}
}

func TestValidatePeerCertificateRejectsRevokedSerial(t *testing.T) {
	registry := NewInMemoryRegistry()
	registry.SetStatus("source-1", SourceStatusActive)
	revocations := &stubRevocations{items: map[string]bool{"1001": true}}

	validator := NewValidatorWithRevocation(registry, revocations)
	now := time.Date(2026, 3, 3, 11, 50, 0, 0, time.UTC)
	validator.now = func() time.Time { return now }

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1001),
		Subject:      pkix.Name{CommonName: "source-1"},
		NotBefore:    now.Add(-1 * time.Minute),
		NotAfter:     now.Add(1 * time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	_, err := validator.ValidatePeerCertificate(cert)
	if !errors.Is(err, ErrRevokedCertificate) {
		t.Fatalf("expected revoked-certificate error, got: %v", err)
	}
}

func TestValidatePeerCertificateUsesCRLManager(t *testing.T) {
	registry := NewInMemoryRegistry()
	registry.SetStatus("source-1", SourceStatusActive)
	crlManager := crl.NewManager()

	validator := NewValidatorWithRevocation(registry, crlManager)
	now := time.Date(2026, 3, 3, 11, 50, 0, 0, time.UTC)
	validator.now = func() time.Time { return now }
	crlManager.RevokeAfter("0x03e9", now)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1001),
		Subject:      pkix.Name{CommonName: "source-1"},
		NotBefore:    now.Add(-1 * time.Minute),
		NotAfter:     now.Add(1 * time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	_, err := validator.ValidatePeerCertificate(cert)
	if !errors.Is(err, ErrRevokedCertificate) {
		t.Fatalf("expected revoked-certificate error with crl manager, got: %v", err)
	}
}

type stubRevocations struct {
	items map[string]bool
}

func (s *stubRevocations) IsRevoked(serial string) bool {
	return s.items[serial]
}
