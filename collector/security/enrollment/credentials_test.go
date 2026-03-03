package enrollment

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/bootstrap"
)

func TestExchangeIssuesClientCredential(t *testing.T) {
	tmp := t.TempDir()
	paths, err := bootstrap.EnsureLocalPKI(bootstrap.Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("bootstrap pki failed: %v", err)
	}

	manager := NewManager()
	fixed := time.Date(2026, 3, 3, 11, 40, 0, 0, time.UTC)
	manager.now = func() time.Time { return fixed }

	token, err := manager.IssueToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	ex := NewExchanger(manager, CredentialConfig{PKIPaths: paths, ValidForDays: 30})
	ex.issuer.now = func() time.Time { return fixed }

	bundle, err := ex.ExchangeTokenForCredential(token, "source-123")
	if err != nil {
		t.Fatalf("exchange failed: %v", err)
	}
	if bundle.SourceID != "source-123" {
		t.Fatalf("unexpected source id: %s", bundle.SourceID)
	}
	if len(bundle.CertPEM) == 0 || len(bundle.KeyPEM) == 0 {
		t.Fatalf("credential bundle should include cert and key")
	}

	certBlock, _ := pem.Decode(bundle.CertPEM)
	if certBlock == nil {
		t.Fatalf("invalid cert pem")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("parse issued cert failed: %v", err)
	}
	if cert.Subject.CommonName != "source-123" {
		t.Fatalf("unexpected cert CN: %s", cert.Subject.CommonName)
	}
	if cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Fatalf("cert should be client auth")
	}

	_, err = ex.ExchangeTokenForCredential(token, "source-123")
	if !errors.Is(err, ErrUsedToken) {
		t.Fatalf("expected used-token error on second exchange, got: %v", err)
	}
}

func TestExchangeRejectsInvalidSourceID(t *testing.T) {
	tmp := t.TempDir()
	paths, err := bootstrap.EnsureLocalPKI(bootstrap.Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("bootstrap pki failed: %v", err)
	}

	manager := NewManager()
	token, err := manager.IssueToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	ex := NewExchanger(manager, CredentialConfig{PKIPaths: paths})
	_, err = ex.ExchangeTokenForCredential(token, "bad/source")
	if !errors.Is(err, ErrInvalidSourceID) {
		t.Fatalf("expected invalid source id error, got: %v", err)
	}
}

func TestIssuerIssuesCredentialWithoutEnrollmentToken(t *testing.T) {
	tmp := t.TempDir()
	paths, err := bootstrap.EnsureLocalPKI(bootstrap.Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("bootstrap pki failed: %v", err)
	}

	issuer := NewIssuer(CredentialConfig{PKIPaths: paths, ValidForDays: 1})
	now := time.Date(2026, 3, 3, 11, 45, 0, 0, time.UTC)
	issuer.now = func() time.Time { return now }

	bundle, err := issuer.IssueForSource("source-renew")
	if err != nil {
		t.Fatalf("issue for source failed: %v", err)
	}
	if bundle.SourceID != "source-renew" {
		t.Fatalf("unexpected source id: %s", bundle.SourceID)
	}
}
