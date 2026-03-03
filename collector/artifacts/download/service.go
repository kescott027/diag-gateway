package download

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kescott027/diag-gateway/collector/artifacts/upload"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

var (
	ErrInvalidToken        = errors.New("invalid download token")
	ErrExpiredToken        = errors.New("expired download token")
	ErrArtifactUnavailable = errors.New("artifact unavailable")
)

const defaultLinkTTL = 10 * time.Minute

// Grant is a verified, time-bounded authorization to read one artifact.
type Grant struct {
	SourceID     string    `json:"source_id"`
	ArtifactID   string    `json:"artifact_id"`
	ArtifactPath string    `json:"artifact_path"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type tokenPayload struct {
	SourceID   string `json:"source_id"`
	ArtifactID string `json:"artifact_id"`
	ExpiresAt  int64  `json:"expires_at"`
	Signature  string `json:"signature"`
}

// Service generates and verifies signed artifact download links.
type Service struct {
	dataDir string
	secret  []byte
	ttl     time.Duration
	now     func() time.Time

	uploads *upload.Service
}

func NewService(dataDir string, secret []byte, ttl time.Duration) (*Service, error) {
	if len(secret) < 16 {
		return nil, fmt.Errorf("download signing secret must be at least 16 bytes")
	}
	if dataDir == "" {
		dataDir = "./collector-data"
	}
	if ttl <= 0 {
		ttl = defaultLinkTTL
	}
	return &Service{
		dataDir: dataDir,
		secret:  append([]byte(nil), secret...),
		ttl:     ttl,
		now:     time.Now,
		uploads: upload.NewService(dataDir),
	}, nil
}

func (s *Service) GenerateToken(sourceID, artifactID string) (string, error) {
	meta, err := s.uploads.LoadMetadata(sourceID, artifactID)
	if err != nil {
		return "", fmt.Errorf("load artifact metadata: %w", err)
	}
	if meta.Status != "completed" {
		return "", fmt.Errorf("%w: %s/%s is not completed", ErrArtifactUnavailable, sourceID, artifactID)
	}

	artifactPath, err := layout.ArtifactBinPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(artifactPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s/%s missing artifact payload", ErrArtifactUnavailable, sourceID, artifactID)
		}
		return "", fmt.Errorf("stat artifact payload: %w", err)
	}

	expiresAt := s.now().UTC().Add(s.ttl)
	payload := tokenPayload{
		SourceID:   sourceID,
		ArtifactID: artifactID,
		ExpiresAt:  expiresAt.Unix(),
	}
	payload.Signature = s.sign(payload)

	blob, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode token payload: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(blob), nil
}

func (s *Service) VerifyToken(token string) (Grant, error) {
	payload, err := s.decodeAndVerifyToken(token)
	if err != nil {
		return Grant{}, err
	}

	artifactPath, err := layout.ArtifactBinPath(s.dataDir, payload.SourceID, payload.ArtifactID)
	if err != nil {
		return Grant{}, err
	}
	if _, err := os.Stat(artifactPath); err != nil {
		if os.IsNotExist(err) {
			return Grant{}, fmt.Errorf("%w: %s/%s missing artifact payload", ErrArtifactUnavailable, payload.SourceID, payload.ArtifactID)
		}
		return Grant{}, fmt.Errorf("stat artifact payload: %w", err)
	}

	meta, err := s.uploads.LoadMetadata(payload.SourceID, payload.ArtifactID)
	if err != nil {
		return Grant{}, fmt.Errorf("load artifact metadata: %w", err)
	}
	if meta.Status != "completed" {
		return Grant{}, fmt.Errorf("%w: %s/%s is not completed", ErrArtifactUnavailable, payload.SourceID, payload.ArtifactID)
	}

	return Grant{
		SourceID:     payload.SourceID,
		ArtifactID:   payload.ArtifactID,
		ArtifactPath: artifactPath,
		ExpiresAt:    time.Unix(payload.ExpiresAt, 0).UTC(),
	}, nil
}

func (s *Service) decodeAndVerifyToken(token string) (tokenPayload, error) {
	if token == "" {
		return tokenPayload{}, ErrInvalidToken
	}
	blob, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return tokenPayload{}, fmt.Errorf("%w: decode token: %v", ErrInvalidToken, err)
	}

	var payload tokenPayload
	if err := json.Unmarshal(blob, &payload); err != nil {
		return tokenPayload{}, fmt.Errorf("%w: decode payload: %v", ErrInvalidToken, err)
	}
	if payload.SourceID == "" || payload.ArtifactID == "" || payload.ExpiresAt <= 0 || payload.Signature == "" {
		return tokenPayload{}, fmt.Errorf("%w: missing fields", ErrInvalidToken)
	}

	expectedSig := s.sign(tokenPayload{
		SourceID:   payload.SourceID,
		ArtifactID: payload.ArtifactID,
		ExpiresAt:  payload.ExpiresAt,
	})
	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(payload.Signature)) != 1 {
		return tokenPayload{}, fmt.Errorf("%w: bad signature", ErrInvalidToken)
	}
	if s.now().UTC().After(time.Unix(payload.ExpiresAt, 0).UTC()) {
		return tokenPayload{}, ErrExpiredToken
	}
	return payload, nil
}

func (s *Service) sign(payload tokenPayload) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload.SourceID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(payload.ArtifactID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(fmt.Sprintf("%d", payload.ExpiresAt)))
	return hex.EncodeToString(mac.Sum(nil))
}
