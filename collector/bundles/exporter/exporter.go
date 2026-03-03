package exporter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kescott027/diag-gateway/collector/artifacts/upload"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
	"github.com/kescott027/diag-gateway/collector/stream/reassembly"
)

// Request controls debug bundle export scope.
type Request struct {
	SourceID    string
	OutputPath  string
	WindowStart time.Time
	WindowEnd   time.Time
}

// ManifestEntry records one file included in the exported bundle.
type ManifestEntry struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Category string `json:"category"`
}

// Manifest is written both inside and alongside a bundle.
type Manifest struct {
	SourceID    string          `json:"source_id"`
	WindowStart string          `json:"window_start,omitempty"`
	WindowEnd   string          `json:"window_end,omitempty"`
	Entries     []ManifestEntry `json:"entries"`
}

// Result summarizes one bundle export.
type Result struct {
	BundlePath   string `json:"bundle_path"`
	ManifestPath string `json:"manifest_path"`
	Files        int    `json:"files"`
	Bytes        int64  `json:"bytes"`
	SHA256       string `json:"sha256"`
}

type fileEntry struct {
	absPath  string
	relPath  string
	category string
}

// Service exports deterministic debug bundles from collector storage.
type Service struct {
	dataDir string
}

func NewService(dataDir string) *Service {
	if dataDir == "" {
		dataDir = "./collector-data"
	}
	return &Service{dataDir: dataDir}
}

func (s *Service) Export(req Request) (Result, error) {
	if strings.TrimSpace(req.SourceID) == "" {
		return Result{}, fmt.Errorf("source_id is required")
	}
	sourceRoot, err := layout.SourceRoot(s.dataDir, req.SourceID)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(sourceRoot); err != nil {
		return Result{}, fmt.Errorf("source root not found: %w", err)
	}

	outputPath := req.OutputPath
	if strings.TrimSpace(outputPath) == "" {
		outputPath = filepath.Join(s.dataDir, "bundles", req.SourceID+".tar.gz")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return Result{}, fmt.Errorf("create bundle output dir: %w", err)
	}

	entries, err := s.collectEntries(req)
	if err != nil {
		return Result{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].relPath < entries[j].relPath })

	manifest := Manifest{SourceID: req.SourceID, Entries: make([]ManifestEntry, 0, len(entries))}
	if !req.WindowStart.IsZero() {
		manifest.WindowStart = req.WindowStart.UTC().Format(time.RFC3339)
	}
	if !req.WindowEnd.IsZero() {
		manifest.WindowEnd = req.WindowEnd.UTC().Format(time.RFC3339)
	}

	bundleFile, err := os.Create(outputPath)
	if err != nil {
		return Result{}, fmt.Errorf("create bundle file: %w", err)
	}
	defer bundleFile.Close()

	gzw := gzip.NewWriter(bundleFile)
	gzw.Header.ModTime = time.Unix(0, 0)
	gzw.Header.OS = 255
	tw := tar.NewWriter(gzw)

	var totalBytes int64
	for _, entry := range entries {
		content, err := os.ReadFile(entry.absPath)
		if err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return Result{}, fmt.Errorf("read bundle entry %s: %w", entry.absPath, err)
		}
		digest := sha256.Sum256(content)
		manifest.Entries = append(manifest.Entries, ManifestEntry{
			Path:     entry.relPath,
			Size:     int64(len(content)),
			SHA256:   hex.EncodeToString(digest[:]),
			Category: entry.category,
		})

		hdr := &tar.Header{
			Name:    entry.relPath,
			Mode:    0o600,
			Size:    int64(len(content)),
			ModTime: time.Unix(0, 0),
			Format:  tar.FormatPAX,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return Result{}, fmt.Errorf("write tar header %s: %w", entry.relPath, err)
		}
		if _, err := tw.Write(content); err != nil {
			_ = tw.Close()
			_ = gzw.Close()
			return Result{}, fmt.Errorf("write tar content %s: %w", entry.relPath, err)
		}
		totalBytes += int64(len(content))
	}

	manifestBlob, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return Result{}, fmt.Errorf("marshal manifest: %w", err)
	}
	manifestHeader := &tar.Header{
		Name:    "manifest.json",
		Mode:    0o600,
		Size:    int64(len(manifestBlob)),
		ModTime: time.Unix(0, 0),
		Format:  tar.FormatPAX,
	}
	if err := tw.WriteHeader(manifestHeader); err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return Result{}, fmt.Errorf("write manifest header: %w", err)
	}
	if _, err := tw.Write(manifestBlob); err != nil {
		_ = tw.Close()
		_ = gzw.Close()
		return Result{}, fmt.Errorf("write manifest payload: %w", err)
	}

	if err := tw.Close(); err != nil {
		_ = gzw.Close()
		return Result{}, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return Result{}, fmt.Errorf("close gzip writer: %w", err)
	}

	manifestPath := outputPath + ".manifest.json"
	if err := os.WriteFile(manifestPath, manifestBlob, 0o600); err != nil {
		return Result{}, fmt.Errorf("write sidecar manifest: %w", err)
	}

	bundleHash, err := fileSHA256(outputPath)
	if err != nil {
		return Result{}, err
	}

	return Result{
		BundlePath:   outputPath,
		ManifestPath: manifestPath,
		Files:        len(entries),
		Bytes:        totalBytes,
		SHA256:       bundleHash,
	}, nil
}

func (s *Service) collectEntries(req Request) ([]fileEntry, error) {
	entries := make([]fileEntry, 0, 64)

	streamsDir := filepath.Join(s.dataDir, "sources", req.SourceID, "streams")
	streamDirs, err := os.ReadDir(streamsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read streams dir: %w", err)
	}
	for _, d := range streamDirs {
		if !d.IsDir() {
			continue
		}
		streamID := d.Name()
		metaPath, err := layout.StreamMetadataPath(s.dataDir, req.SourceID, streamID)
		if err != nil {
			continue
		}
		meta, err := readJSON[reassembly.Metadata](metaPath)
		if err != nil {
			continue
		}
		include := withinWindow(meta.CreatedAt, req.WindowStart, req.WindowEnd)
		if !include {
			continue
		}
		logPath, err := layout.StreamLogPath(s.dataDir, req.SourceID, streamID)
		if err == nil {
			if _, statErr := os.Stat(logPath); statErr == nil {
				entries = append(entries, fileEntry{absPath: logPath, relPath: relForBundle(s.dataDir, logPath), category: "stream-log"})
			}
		}
		if _, statErr := os.Stat(metaPath); statErr == nil {
			entries = append(entries, fileEntry{absPath: metaPath, relPath: relForBundle(s.dataDir, metaPath), category: "stream-meta"})
		}
	}

	artifactsDir := filepath.Join(s.dataDir, "sources", req.SourceID, "artifacts")
	artifactDirs, err := os.ReadDir(artifactsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read artifacts dir: %w", err)
	}
	for _, d := range artifactDirs {
		if !d.IsDir() {
			continue
		}
		artifactID := d.Name()
		metaPath, err := layout.ArtifactMetadataPath(s.dataDir, req.SourceID, artifactID)
		if err != nil {
			continue
		}
		meta, err := readJSON[upload.Metadata](metaPath)
		if err != nil {
			continue
		}
		timeRef := meta.CompletedAt
		if timeRef == "" {
			timeRef = meta.UploadTimestamp
		}
		if timeRef == "" {
			timeRef = meta.CreatedAt
		}
		if !withinWindow(timeRef, req.WindowStart, req.WindowEnd) {
			continue
		}
		artifactPath, err := layout.ArtifactBinPath(s.dataDir, req.SourceID, artifactID)
		if err == nil {
			if _, statErr := os.Stat(artifactPath); statErr == nil {
				entries = append(entries, fileEntry{absPath: artifactPath, relPath: relForBundle(s.dataDir, artifactPath), category: "artifact-bin"})
			}
		}
		if _, statErr := os.Stat(metaPath); statErr == nil {
			entries = append(entries, fileEntry{absPath: metaPath, relPath: relForBundle(s.dataDir, metaPath), category: "artifact-meta"})
		}
	}

	return entries, nil
}

func relForBundle(dataDir, absPath string) string {
	rel, err := filepath.Rel(dataDir, absPath)
	if err != nil {
		return filepath.Base(absPath)
	}
	return filepath.ToSlash(rel)
}

func withinWindow(timestamp string, start, end time.Time) bool {
	if timestamp == "" {
		return start.IsZero() && end.IsZero()
	}
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return false
	}
	if !start.IsZero() && ts.Before(start) {
		return false
	}
	if !end.IsZero() && ts.After(end) {
		return false
	}
	return true
}

func fileSHA256(path string) (string, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file hash input: %w", err)
	}
	digest := sha256.Sum256(blob)
	return hex.EncodeToString(digest[:]), nil
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

// DecodeBundleManifest is a small helper used by tests and future APIs.
func DecodeBundleManifest(blob []byte) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(blob))
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ReadTarGzManifest extracts manifest.json from a generated bundle.
func ReadTarGzManifest(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return Manifest{}, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Manifest{}, err
		}
		if hdr.Name != "manifest.json" {
			continue
		}
		blob, err := io.ReadAll(tr)
		if err != nil {
			return Manifest{}, err
		}
		return DecodeBundleManifest(blob)
	}
	return Manifest{}, fmt.Errorf("manifest.json not found in bundle")
}
