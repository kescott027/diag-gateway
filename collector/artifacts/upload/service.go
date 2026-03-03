package upload

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

var ErrOffsetMismatch = errors.New("artifact append offset mismatch")
var ErrArtifactCompleted = errors.New("artifact already completed")

// Metadata tracks artifact lifecycle and integrity state.
type Metadata struct {
	SourceID     string `json:"source_id"`
	ArtifactID   string `json:"artifact_id"`
	FileName     string `json:"file_name,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256,omitempty"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	CompletedAt  string `json:"completed_at,omitempty"`
	LastAppendAt string `json:"last_append_at,omitempty"`
}

// BeginOptions captures optional metadata provided at artifact creation.
type BeginOptions struct {
	FileName    string
	ContentType string
}

// ResumeState describes upload continuation status for one artifact.
type ResumeState struct {
	SourceID   string `json:"source_id"`
	ArtifactID string `json:"artifact_id"`
	Status     string `json:"status"`
	NextOffset int64  `json:"next_offset"`
	CanResume  bool   `json:"can_resume"`
}

// Service manages durable append-oriented artifact uploads.
type Service struct {
	dataDir string
	now     func() time.Time
}

func NewService(dataDir string) *Service {
	if dataDir == "" {
		dataDir = "./collector-data"
	}
	return &Service{dataDir: dataDir, now: time.Now}
}

func (s *Service) Begin(sourceID, artifactID string, options BeginOptions) (Metadata, error) {
	artifactBin, err := layout.ArtifactBinPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return Metadata{}, err
	}
	artifactMeta, err := layout.ArtifactMetadataPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return Metadata{}, err
	}

	if err := os.MkdirAll(filepath.Dir(artifactBin), 0o700); err != nil {
		return Metadata{}, fmt.Errorf("create artifact dir: %w", err)
	}

	if err := ensureFile(artifactBin, 0o600); err != nil {
		return Metadata{}, err
	}

	size, err := fileSize(artifactBin)
	if err != nil {
		return Metadata{}, err
	}

	meta, err := s.loadMetadata(sourceID, artifactID)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Metadata{}, err
		}
		meta = Metadata{}
	}
	if meta.Status == "completed" {
		return Metadata{}, fmt.Errorf("%w: %s/%s", ErrArtifactCompleted, sourceID, artifactID)
	}

	now := s.now().UTC().Format(time.RFC3339)
	if meta.CreatedAt == "" {
		meta.CreatedAt = now
	}
	meta.SourceID = sourceID
	meta.ArtifactID = artifactID
	meta.FileName = options.FileName
	meta.ContentType = options.ContentType
	meta.SizeBytes = size
	meta.Status = "open"
	meta.LastAppendAt = ""
	meta.CompletedAt = ""
	meta.SHA256 = ""

	if err := writeMetadataAtomic(artifactMeta, meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func (s *Service) Append(sourceID, artifactID string, offset int64, payload []byte) (int64, error) {
	artifactBin, err := layout.ArtifactBinPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return 0, err
	}
	if err := ensureFile(artifactBin, 0o600); err != nil {
		return 0, err
	}

	currentSize, err := fileSize(artifactBin)
	if err != nil {
		return 0, err
	}
	if offset != currentSize {
		return currentSize, fmt.Errorf("%w: expected=%d got=%d", ErrOffsetMismatch, currentSize, offset)
	}

	f, err := os.OpenFile(artifactBin, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return currentSize, fmt.Errorf("open artifact file: %w", err)
	}
	defer f.Close()

	n, err := f.Write(payload)
	if err != nil {
		return currentSize, fmt.Errorf("append artifact payload: %w", err)
	}
	newSize := currentSize + int64(n)

	meta, err := s.loadMetadata(sourceID, artifactID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			meta, err = s.Begin(sourceID, artifactID, BeginOptions{})
			if err != nil {
				return currentSize, err
			}
		} else {
			return currentSize, err
		}
	}
	if meta.Status == "completed" {
		return currentSize, fmt.Errorf("%w: %s/%s", ErrArtifactCompleted, sourceID, artifactID)
	}

	meta.SizeBytes = newSize
	meta.Status = "open"
	meta.LastAppendAt = s.now().UTC().Format(time.RFC3339)
	artifactMeta, err := layout.ArtifactMetadataPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return currentSize, err
	}
	if err := writeMetadataAtomic(artifactMeta, meta); err != nil {
		return currentSize, err
	}

	return newSize, nil
}

func (s *Service) Finalize(sourceID, artifactID string) (Metadata, error) {
	artifactBin, err := layout.ArtifactBinPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return Metadata{}, err
	}
	artifactMeta, err := layout.ArtifactMetadataPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return Metadata{}, err
	}

	meta, err := s.loadMetadata(sourceID, artifactID)
	if err != nil {
		return Metadata{}, err
	}
	if meta.Status == "completed" {
		return meta, nil
	}

	hash, size, err := fileHashAndSize(artifactBin)
	if err != nil {
		return Metadata{}, err
	}
	meta.SizeBytes = size
	meta.SHA256 = hash
	meta.Status = "completed"
	meta.CompletedAt = s.now().UTC().Format(time.RFC3339)

	if err := writeMetadataAtomic(artifactMeta, meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func (s *Service) LoadMetadata(sourceID, artifactID string) (Metadata, error) {
	return s.loadMetadata(sourceID, artifactID)
}

func (s *Service) ResumeState(sourceID, artifactID string) (ResumeState, error) {
	artifactBin, err := layout.ArtifactBinPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return ResumeState{}, err
	}

	size := int64(0)
	if info, statErr := os.Stat(artifactBin); statErr == nil {
		size = info.Size()
	} else if !os.IsNotExist(statErr) {
		return ResumeState{}, fmt.Errorf("stat artifact for resume: %w", statErr)
	}

	meta, err := s.loadMetadata(sourceID, artifactID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ResumeState{
				SourceID:   sourceID,
				ArtifactID: artifactID,
				Status:     "new",
				NextOffset: size,
				CanResume:  true,
			}, nil
		}
		return ResumeState{}, err
	}

	nextOffset := meta.SizeBytes
	if nextOffset < size {
		nextOffset = size
	}
	status := meta.Status
	if status == "" {
		status = "open"
	}
	return ResumeState{
		SourceID:   sourceID,
		ArtifactID: artifactID,
		Status:     status,
		NextOffset: nextOffset,
		CanResume:  status != "completed",
	}, nil
}

func (s *Service) loadMetadata(sourceID, artifactID string) (Metadata, error) {
	artifactMeta, err := layout.ArtifactMetadataPath(s.dataDir, sourceID, artifactID)
	if err != nil {
		return Metadata{}, err
	}
	blob, err := os.ReadFile(artifactMeta)
	if err != nil {
		return Metadata{}, err
	}
	var meta Metadata
	if err := json.Unmarshal(blob, &meta); err != nil {
		return Metadata{}, fmt.Errorf("decode artifact metadata: %w", err)
	}
	return meta, nil
}

func writeMetadataAtomic(path string, meta Metadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create artifact metadata dir: %w", err)
	}
	blob, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode artifact metadata: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return fmt.Errorf("write artifact metadata temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("promote artifact metadata: %w", err)
	}
	return nil
}

func ensureFile(path string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ensure artifact dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE, mode)
	if err != nil {
		return fmt.Errorf("ensure artifact file: %w", err)
	}
	return f.Close()
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat artifact file: %w", err)
	}
	return info.Size(), nil
}

func fileHashAndSize(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open artifact file for hash: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("hash artifact file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
