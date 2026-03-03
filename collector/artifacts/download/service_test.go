package download

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/artifacts/upload"
)

func TestGenerateAndVerifyToken(t *testing.T) {
	tmp := t.TempDir()
	up := upload.NewService(tmp)
	if _, err := up.Begin("source-1", "artifact-1", upload.BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if _, err := up.Append("source-1", "artifact-1", 0, []byte("payload")); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := up.Finalize("source-1", "artifact-1"); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	svc, err := NewService(tmp, []byte("0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	token, err := svc.GenerateToken("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	grant, err := svc.VerifyToken(token)
	if err != nil {
		t.Fatalf("verify token failed: %v", err)
	}
	if grant.SourceID != "source-1" || grant.ArtifactID != "artifact-1" {
		t.Fatalf("unexpected grant: %+v", grant)
	}
	if _, err := os.Stat(grant.ArtifactPath); err != nil {
		t.Fatalf("expected artifact path to exist, got %v", err)
	}
}

func TestGenerateTokenRejectsOpenArtifact(t *testing.T) {
	tmp := t.TempDir()
	up := upload.NewService(tmp)
	if _, err := up.Begin("source-1", "artifact-1", upload.BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}

	svc, err := NewService(tmp, []byte("0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	if _, err := svc.GenerateToken("source-1", "artifact-1"); !errors.Is(err, ErrArtifactUnavailable) {
		t.Fatalf("expected ErrArtifactUnavailable, got %v", err)
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	tmp := t.TempDir()
	up := upload.NewService(tmp)
	if _, err := up.Begin("source-1", "artifact-1", upload.BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if _, err := up.Append("source-1", "artifact-1", 0, []byte("payload")); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := up.Finalize("source-1", "artifact-1"); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	svc, err := NewService(tmp, []byte("0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	svc.now = func() time.Time { return time.Unix(2000, 0).UTC() }

	payload := tokenPayload{
		SourceID:   "source-1",
		ArtifactID: "artifact-1",
		ExpiresAt:  time.Unix(1000, 0).UTC().Unix(),
	}
	payload.Signature = svc.sign(payload)
	blob, _ := json.Marshal(payload)
	token := base64.RawURLEncoding.EncodeToString(blob)

	if _, err := svc.VerifyToken(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestVerifyTokenInvalidSignature(t *testing.T) {
	tmp := t.TempDir()
	svc, err := NewService(tmp, []byte("0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	payload := tokenPayload{
		SourceID:   "source-1",
		ArtifactID: "artifact-1",
		ExpiresAt:  time.Now().Add(time.Hour).Unix(),
		Signature:  "badsignature",
	}
	blob, _ := json.Marshal(payload)
	token := base64.RawURLEncoding.EncodeToString(blob)

	if _, err := svc.VerifyToken(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyTokenMissingArtifact(t *testing.T) {
	tmp := t.TempDir()
	up := upload.NewService(tmp)
	if _, err := up.Begin("source-1", "artifact-1", upload.BeginOptions{}); err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if _, err := up.Append("source-1", "artifact-1", 0, []byte("payload")); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := up.Finalize("source-1", "artifact-1"); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	svc, err := NewService(tmp, []byte("0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	token, err := svc.GenerateToken("source-1", "artifact-1")
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	if err := os.Remove(fmt.Sprintf("%s/sources/source-1/artifacts/artifact-1/artifact.bin", tmp)); err != nil {
		t.Fatalf("remove artifact failed: %v", err)
	}
	if _, err := svc.VerifyToken(token); !errors.Is(err, ErrArtifactUnavailable) {
		t.Fatalf("expected ErrArtifactUnavailable, got %v", err)
	}
}
