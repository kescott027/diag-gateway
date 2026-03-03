package retention

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kescott027/diag-gateway/collector/artifacts/upload"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
	"github.com/kescott027/diag-gateway/collector/stream/reassembly"
)

// Policy defines per-source retention windows in days.
type Policy struct {
	SourceID              string `json:"source_id"`
	StreamRetentionDays   int    `json:"stream_retention_days"`
	ArtifactRetentionDays int    `json:"artifact_retention_days"`
	UpdatedAt             string `json:"updated_at,omitempty"`
}

// Result summarizes one prune run.
type Result struct {
	SourceID             string `json:"source_id"`
	StreamsDeleted       int    `json:"streams_deleted"`
	StreamsSkippedActive int    `json:"streams_skipped_active"`
	ArtifactsDeleted     int    `json:"artifacts_deleted"`
	ArtifactsSkippedOpen int    `json:"artifacts_skipped_open"`
}

// Service stores policies and applies retention pruning.
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

func (s *Service) SetPolicy(policy Policy) (Policy, error) {
	if strings.TrimSpace(policy.SourceID) == "" {
		return Policy{}, fmt.Errorf("source_id is required")
	}
	if policy.StreamRetentionDays < 0 || policy.ArtifactRetentionDays < 0 {
		return Policy{}, fmt.Errorf("retention days must be >= 0")
	}

	if _, err := layout.SourceRoot(s.dataDir, policy.SourceID); err != nil {
		return Policy{}, err
	}
	policy.UpdatedAt = s.now().UTC().Format(time.RFC3339)

	path, err := s.policyPath(policy.SourceID)
	if err != nil {
		return Policy{}, err
	}
	if err := writeAtomicJSON(path, policy); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func (s *Service) GetPolicy(sourceID string) (Policy, error) {
	path, err := s.policyPath(sourceID)
	if err != nil {
		return Policy{}, err
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Policy{SourceID: sourceID}, nil
		}
		return Policy{}, fmt.Errorf("read retention policy: %w", err)
	}
	var p Policy
	if err := json.Unmarshal(blob, &p); err != nil {
		return Policy{}, fmt.Errorf("decode retention policy: %w", err)
	}
	if p.SourceID == "" {
		p.SourceID = sourceID
	}
	return p, nil
}

func (s *Service) PruneSource(sourceID string) (Result, error) {
	policy, err := s.GetPolicy(sourceID)
	if err != nil {
		return Result{}, err
	}

	res := Result{SourceID: sourceID}
	if policy.StreamRetentionDays > 0 {
		streamsDeleted, streamsSkipped, err := s.pruneStreams(sourceID, policy.StreamRetentionDays)
		if err != nil {
			return Result{}, err
		}
		res.StreamsDeleted = streamsDeleted
		res.StreamsSkippedActive = streamsSkipped
	}
	if policy.ArtifactRetentionDays > 0 {
		artDeleted, artSkipped, err := s.pruneArtifacts(sourceID, policy.ArtifactRetentionDays)
		if err != nil {
			return Result{}, err
		}
		res.ArtifactsDeleted = artDeleted
		res.ArtifactsSkippedOpen = artSkipped
	}
	return res, nil
}

func (s *Service) pruneStreams(sourceID string, days int) (int, int, error) {
	sourceRoot, err := layout.SourceRoot(s.dataDir, sourceID)
	if err != nil {
		return 0, 0, err
	}
	streamsDir := filepath.Join(sourceRoot, "streams")
	entries, err := os.ReadDir(streamsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("read streams dir: %w", err)
	}

	cutoff := s.now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	deleted := 0
	skippedActive := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		streamID := entry.Name()
		metaPath, err := layout.StreamMetadataPath(s.dataDir, sourceID, streamID)
		if err != nil {
			continue
		}
		meta, err := readJSON[reassembly.Metadata](metaPath)
		if err != nil {
			continue
		}
		if strings.ToLower(meta.Status) != "closed" {
			skippedActive++
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, meta.CreatedAt)
		if err != nil || !createdAt.Before(cutoff) {
			continue
		}
		streamDir, err := layout.StreamDir(s.dataDir, sourceID, streamID)
		if err != nil {
			continue
		}
		if err := os.RemoveAll(streamDir); err != nil {
			return deleted, skippedActive, fmt.Errorf("delete stream dir %s: %w", streamDir, err)
		}
		deleted++
	}
	return deleted, skippedActive, nil
}

func (s *Service) pruneArtifacts(sourceID string, days int) (int, int, error) {
	sourceRoot, err := layout.SourceRoot(s.dataDir, sourceID)
	if err != nil {
		return 0, 0, err
	}
	artifactsDir := filepath.Join(sourceRoot, "artifacts")
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("read artifacts dir: %w", err)
	}

	cutoff := s.now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	deleted := 0
	skippedOpen := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		artifactID := entry.Name()
		metaPath, err := layout.ArtifactMetadataPath(s.dataDir, sourceID, artifactID)
		if err != nil {
			continue
		}
		meta, err := readJSON[upload.Metadata](metaPath)
		if err != nil {
			continue
		}
		if strings.ToLower(meta.Status) != "completed" {
			skippedOpen++
			continue
		}

		referenceTime, err := retentionReferenceTime(meta)
		if err != nil || !referenceTime.Before(cutoff) {
			continue
		}
		artifactDir, err := layout.ArtifactDir(s.dataDir, sourceID, artifactID)
		if err != nil {
			continue
		}
		if err := os.RemoveAll(artifactDir); err != nil {
			return deleted, skippedOpen, fmt.Errorf("delete artifact dir %s: %w", artifactDir, err)
		}
		deleted++
	}
	return deleted, skippedOpen, nil
}

func retentionReferenceTime(meta upload.Metadata) (time.Time, error) {
	if meta.CompletedAt != "" {
		return time.Parse(time.RFC3339, meta.CompletedAt)
	}
	if meta.UploadTimestamp != "" {
		return time.Parse(time.RFC3339, meta.UploadTimestamp)
	}
	return time.Parse(time.RFC3339, meta.CreatedAt)
}

func (s *Service) policyPath(sourceID string) (string, error) {
	sourceRoot, err := layout.SourceRoot(s.dataDir, sourceID)
	if err != nil {
		return "", err
	}
	return filepath.Join(sourceRoot, "retention.json"), nil
}

func writeAtomicJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	blob, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json for %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return fmt.Errorf("write temp file %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("promote file %s: %w", path, err)
	}
	return nil
}

func readJSON[T any](path string) (T, error) {
	var out T
	blob, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(blob, &out); err != nil {
		return out, err
	}
	return out, nil
}
