package rotation

import (
	"crypto/x509"
	"os"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/bootstrap"
)

func TestShouldRenewWindow(t *testing.T) {
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	cert := &x509.Certificate{NotAfter: now.Add(5 * 24 * time.Hour)}

	renew, err := ShouldRenew(cert, now, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("should renew check failed: %v", err)
	}
	if !renew {
		t.Fatalf("expected renewal when inside window")
	}

	renew, err = ShouldRenew(cert, now, 2*24*time.Hour)
	if err != nil {
		t.Fatalf("should renew check failed: %v", err)
	}
	if renew {
		t.Fatalf("did not expect renewal when outside window")
	}
}

func TestRotateServerCertificateIfNeeded(t *testing.T) {
	tmp := t.TempDir()
	paths, err := bootstrap.EnsureLocalPKI(bootstrap.Config{OutputDir: tmp})
	if err != nil {
		t.Fatalf("bootstrap pki failed: %v", err)
	}

	rotator := NewRotator(paths, "collector.local", Policy{RenewBefore: 1000 * 24 * time.Hour, ValidForDays: 30})
	fixed := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	rotator.now = func() time.Time { return fixed }

	original, err := os.ReadFile(paths.ServerCertPath)
	if err != nil {
		t.Fatalf("read original server cert: %v", err)
	}

	rotated, err := rotator.RotateServerCertificateIfNeeded()
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}
	if !rotated {
		t.Fatalf("expected rotation to occur")
	}

	replaced, err := os.ReadFile(paths.ServerCertPath)
	if err != nil {
		t.Fatalf("read rotated server cert: %v", err)
	}
	if string(original) == string(replaced) {
		t.Fatalf("server cert was not replaced")
	}
}
