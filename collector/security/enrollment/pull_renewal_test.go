package enrollment

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/admission"
	"github.com/kescott027/diag-gateway/collector/security/bootstrap"
)

func TestPullRenewIfNeededIssuesRotatedCredential(t *testing.T) {
	tmp := t.TempDir()
	paths, err := bootstrap.EnsureLocalPKI(bootstrap.Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("bootstrap pki failed: %v", err)
	}

	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)

	oldIssuer := NewIssuer(CredentialConfig{PKIPaths: paths, ValidForDays: 1})
	oldIssuer.now = func() time.Time { return now }
	oldBundle, err := oldIssuer.IssueForSource("source-1")
	if err != nil {
		t.Fatalf("issue old credential failed: %v", err)
	}
	oldCert := mustParseCert(t, oldBundle.CertPEM)
	oldCert.SerialNumber = big.NewInt(4242)

	registry := admission.NewInMemoryRegistry()
	registry.SetStatus("source-1", admission.SourceStatusActive)
	revocations := NewSerialRevocations()
	revocations.now = func() time.Time { return now }
	validator := admission.NewValidatorWithRevocation(registry, revocations)
	validator.SetNow(func() time.Time { return now })

	newIssuer := NewIssuer(CredentialConfig{PKIPaths: paths, ValidForDays: 30})
	newIssuer.now = func() time.Time { return now }

	renewer := NewPullRenewer(validator, newIssuer, revocations, PullRenewalPolicy{
		RenewBefore:   48 * time.Hour,
		MinIssueGap:   1 * time.Minute,
		OverlapWindow: 10 * time.Minute,
	})
	renewer.now = func() time.Time { return now }

	bundle, rotated, err := renewer.PullRenewIfNeeded(oldCert)
	if err != nil {
		t.Fatalf("pull renewal failed: %v", err)
	}
	if !rotated {
		t.Fatalf("expected renewal to rotate credential")
	}
	if bundle.SourceID != "source-1" {
		t.Fatalf("unexpected source id: %s", bundle.SourceID)
	}
	if revocations.IsRevoked(oldCert.SerialNumber.String()) {
		t.Fatalf("old cert should remain valid during overlap window")
	}

	nowLater := now.Add(11 * time.Minute)
	revocations.now = func() time.Time { return nowLater }
	validator.SetNow(func() time.Time { return nowLater })
	_, err = validator.ValidatePeerCertificate(oldCert)
	if !errors.Is(err, admission.ErrRevokedCertificate) {
		t.Fatalf("expected old cert to be revoked after overlap, got: %v", err)
	}
}

func TestPullRenewIfNeededNoopWhenOutsideWindow(t *testing.T) {
	tmp := t.TempDir()
	paths, err := bootstrap.EnsureLocalPKI(bootstrap.Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("bootstrap pki failed: %v", err)
	}

	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)

	issuer := NewIssuer(CredentialConfig{PKIPaths: paths, ValidForDays: 30})
	issuer.now = func() time.Time { return now }
	bundle, err := issuer.IssueForSource("source-2")
	if err != nil {
		t.Fatalf("issue credential failed: %v", err)
	}
	cert := mustParseCert(t, bundle.CertPEM)

	registry := admission.NewInMemoryRegistry()
	registry.SetStatus("source-2", admission.SourceStatusActive)
	validator := admission.NewValidator(registry)
	validator.SetNow(func() time.Time { return now })

	renewer := NewPullRenewer(validator, issuer, nil, PullRenewalPolicy{RenewBefore: 1 * time.Hour})
	renewer.now = func() time.Time { return now }

	_, rotated, err := renewer.PullRenewIfNeeded(cert)
	if err != nil {
		t.Fatalf("pull renewal failed: %v", err)
	}
	if rotated {
		t.Fatalf("did not expect rotation when outside renewal window")
	}
}

func mustParseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatalf("invalid cert pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert failed: %v", err)
	}
	return cert
}
