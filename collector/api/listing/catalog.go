package listing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

// FileEntry is a UI-ready listing row for one stream-backed log file.
type FileEntry struct {
	SourceID    string `json:"source_id"`
	StreamID    string `json:"stream_id"`
	LogicalPath string `json:"logical_path"`
	LastOffset  int64  `json:"last_offset"`
	Status      string `json:"status"`
}

// SourceCatalog groups stream files by source.
type SourceCatalog struct {
	SourceID string      `json:"source_id"`
	Files    []FileEntry `json:"files"`
}

// Catalog is the deterministic source/file listing payload for control-plane UI.
type Catalog struct {
	Sources []SourceCatalog `json:"sources"`
}

type streamMetadata struct {
	LogicalPath string `json:"logical_path"`
	LastOffset  int64  `json:"last_offset"`
	Status      string `json:"status"`
}

// Service enumerates source and stream state from collector storage layout.
type Service struct {
	dataDir string
}

func NewService(dataDir string) *Service {
	if dataDir == "" {
		dataDir = "./collector-data"
	}
	return &Service{dataDir: dataDir}
}

func (s *Service) ListSources() ([]string, error) {
	root := filepath.Join(s.dataDir, "sources")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read sources directory: %w", err)
	}

	sources := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourceID := entry.Name()
		if _, err := layout.SourceRoot(s.dataDir, sourceID); err != nil {
			continue
		}
		sources = append(sources, sourceID)
	}

	sort.Strings(sources)
	return sources, nil
}

func (s *Service) ListSourceFiles(sourceID string) ([]FileEntry, error) {
	sourceRoot, err := layout.SourceRoot(s.dataDir, sourceID)
	if err != nil {
		return nil, err
	}

	streamsDir := filepath.Join(sourceRoot, "streams")
	entries, err := os.ReadDir(streamsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []FileEntry{}, nil
		}
		return nil, fmt.Errorf("read streams directory for %s: %w", sourceID, err)
	}

	files := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		streamID := entry.Name()
		metaPath, err := layout.StreamMetadataPath(s.dataDir, sourceID, streamID)
		if err != nil {
			continue
		}

		var meta streamMetadata
		blob, err := os.ReadFile(metaPath)
		if err == nil {
			_ = json.Unmarshal(blob, &meta)
		}
		if meta.LogicalPath == "" {
			meta.LogicalPath = streamID
		}
		if meta.Status == "" {
			meta.Status = "unknown"
		}

		files = append(files, FileEntry{
			SourceID:    sourceID,
			StreamID:    streamID,
			LogicalPath: meta.LogicalPath,
			LastOffset:  meta.LastOffset,
			Status:      meta.Status,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].LogicalPath == files[j].LogicalPath {
			return files[i].StreamID < files[j].StreamID
		}
		return files[i].LogicalPath < files[j].LogicalPath
	})
	return files, nil
}

func (s *Service) ListCatalog() (Catalog, error) {
	sourceIDs, err := s.ListSources()
	if err != nil {
		return Catalog{}, err
	}

	catalog := Catalog{Sources: make([]SourceCatalog, 0, len(sourceIDs))}
	for _, sourceID := range sourceIDs {
		files, err := s.ListSourceFiles(sourceID)
		if err != nil {
			return Catalog{}, err
		}
		catalog.Sources = append(catalog.Sources, SourceCatalog{
			SourceID: sourceID,
			Files:    files,
		})
	}
	return catalog, nil
}

