package revocation

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/admission"
	"github.com/kescott027/diag-gateway/collector/security/enrollment"
)

func TestRevokeSourceAndSerialWithAudit(t *testing.T) {
	registry := admission.NewInMemoryRegistry()
	registry.SetStatus("source-1", admission.SourceStatusActive)

	serials := enrollment.NewSerialRevocations()
	auditPath := filepath.Join(t.TempDir(), "audit", "revocations.log")
	sink := NewAppendOnlyFileAuditSink(auditPath)

	now := time.Now().UTC()
	svc := NewService(registry, serials, sink)
	svc.now = func() time.Time { return now }

	if err := svc.RevokeSource("source-1", "operator-a", "manual block"); err != nil {
		t.Fatalf("revoke source failed: %v", err)
	}
	if st := registry.GetStatus("source-1"); st != admission.SourceStatusRevoked {
		t.Fatalf("expected source revoked, got %s", st)
	}

	if err := svc.RevokeSerial("0x3039", "operator-a", "suspected leak"); err != nil {
		t.Fatalf("revoke serial failed: %v", err)
	}
	if !serials.IsRevoked("12345") {
		t.Fatalf("expected serial to be revoked immediately")
	}

	file, err := os.Open(auditPath)
	if err != nil {
		t.Fatalf("open audit file failed: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit file failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 audit events, got %d", count)
	}
}
