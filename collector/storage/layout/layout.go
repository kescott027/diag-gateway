package layout

import (
	"fmt"
	"path/filepath"
	"regexp"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validateID(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if !idPattern.MatchString(value) {
		return fmt.Errorf("%s contains invalid characters: %q", name, value)
	}
	return nil
}

// SourceRoot returns deterministic root path for a source.
func SourceRoot(dataDir, sourceID string) (string, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "sources", sourceID), nil
}

// StreamDir returns deterministic stream directory path.
func StreamDir(dataDir, sourceID, streamID string) (string, error) {
	if err := validateID("stream_id", streamID); err != nil {
		return "", err
	}
	sourceRoot, err := SourceRoot(dataDir, sourceID)
	if err != nil {
		return "", err
	}
	return filepath.Join(sourceRoot, "streams", streamID), nil
}

func StreamLogPath(dataDir, sourceID, streamID string) (string, error) {
	dir, err := StreamDir(dataDir, sourceID, streamID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "stream.log"), nil
}

func StreamMetadataPath(dataDir, sourceID, streamID string) (string, error) {
	dir, err := StreamDir(dataDir, sourceID, streamID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "metadata.json"), nil
}

// ArtifactDir returns deterministic artifact directory path.
func ArtifactDir(dataDir, sourceID, artifactID string) (string, error) {
	if err := validateID("artifact_id", artifactID); err != nil {
		return "", err
	}
	sourceRoot, err := SourceRoot(dataDir, sourceID)
	if err != nil {
		return "", err
	}
	return filepath.Join(sourceRoot, "artifacts", artifactID), nil
}

func ArtifactBinPath(dataDir, sourceID, artifactID string) (string, error) {
	dir, err := ArtifactDir(dataDir, sourceID, artifactID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "artifact.bin"), nil
}

func ArtifactMetadataPath(dataDir, sourceID, artifactID string) (string, error) {
	dir, err := ArtifactDir(dataDir, sourceID, artifactID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "metadata.json"), nil
}

func SourceMetricsPath(dataDir, sourceID string) (string, error) {
	sourceRoot, err := SourceRoot(dataDir, sourceID)
	if err != nil {
		return "", err
	}
	return filepath.Join(sourceRoot, "metrics.json"), nil
}
