package reassembly

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

var (
	ErrOffsetMismatch = errors.New("offset mismatch for append-only stream")
)

type streamKey struct {
	sourceID string
	streamID string
}

// Metadata tracks stream path and append progress.
type Metadata struct {
	LogicalPath string `json:"logical_path"`
	CreatedAt   string `json:"created_at"`
	LastOffset  int64  `json:"last_offset"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
}

// Reassembler enforces append-only writes to deterministic stream paths.
type Reassembler struct {
	dataDir string
	now     func() time.Time

	mu    sync.Mutex
	state map[streamKey]int64
}

func NewReassembler(dataDir string) *Reassembler {
	return &Reassembler{
		dataDir: dataDir,
		now:     time.Now,
		state:   make(map[streamKey]int64),
	}
}

func (r *Reassembler) OpenStream(sourceID, streamID, logicalPath string) error {
	streamDir, err := layout.StreamDir(r.dataDir, sourceID, streamID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(streamDir, 0o700); err != nil {
		return fmt.Errorf("create stream dir: %w", err)
	}

	logPath, err := layout.StreamLogPath(r.dataDir, sourceID, streamID)
	if err != nil {
		return err
	}
	if err := ensureFile(logPath, 0o600); err != nil {
		return err
	}

	size, err := fileSize(logPath)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.state[streamKey{sourceID: sourceID, streamID: streamID}] = size
	r.mu.Unlock()

	meta := Metadata{
		LogicalPath: logicalPath,
		CreatedAt:   r.now().UTC().Format(time.RFC3339),
		LastOffset:  size,
		Status:      "open",
	}
	return r.writeMetadata(sourceID, streamID, meta)
}

func (r *Reassembler) AppendChunk(sourceID, streamID string, offset int64, payload []byte) (int64, error) {
	logPath, err := layout.StreamLogPath(r.dataDir, sourceID, streamID)
	if err != nil {
		return 0, err
	}
	if err := ensureFile(logPath, 0o600); err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := streamKey{sourceID: sourceID, streamID: streamID}
	expected, ok := r.state[key]
	if !ok {
		expected, err = fileSize(logPath)
		if err != nil {
			return 0, err
		}
		r.state[key] = expected
	}
	if offset != expected {
		return expected, fmt.Errorf("%w: expected=%d got=%d", ErrOffsetMismatch, expected, offset)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return expected, fmt.Errorf("open stream log: %w", err)
	}
	defer file.Close()

	written, err := file.Write(payload)
	if err != nil {
		return expected, fmt.Errorf("append payload: %w", err)
	}
	newOffset := expected + int64(written)
	r.state[key] = newOffset

	meta, _ := r.readMetadata(sourceID, streamID)
	meta.LastOffset = newOffset
	if meta.Status == "" {
		meta.Status = "open"
	}
	if err := r.writeMetadata(sourceID, streamID, meta); err != nil {
		return expected, err
	}

	return newOffset, nil
}

func (r *Reassembler) CloseStream(sourceID, streamID, reason string) error {
	r.mu.Lock()
	last := r.state[streamKey{sourceID: sourceID, streamID: streamID}]
	r.mu.Unlock()

	meta, _ := r.readMetadata(sourceID, streamID)
	meta.LastOffset = last
	meta.Status = "closed"
	meta.Reason = reason
	return r.writeMetadata(sourceID, streamID, meta)
}

func (r *Reassembler) writeMetadata(sourceID, streamID string, meta Metadata) error {
	metaPath, err := layout.StreamMetadataPath(r.dataDir, sourceID, streamID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o700); err != nil {
		return fmt.Errorf("create metadata dir: %w", err)
	}

	blob, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal stream metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, blob, 0o600); err != nil {
		return fmt.Errorf("write stream metadata: %w", err)
	}
	return nil
}

func (r *Reassembler) readMetadata(sourceID, streamID string) (Metadata, error) {
	metaPath, err := layout.StreamMetadataPath(r.dataDir, sourceID, streamID)
	if err != nil {
		return Metadata{}, err
	}
	blob, err := os.ReadFile(metaPath)
	if err != nil {
		return Metadata{}, err
	}
	var meta Metadata
	if err := json.Unmarshal(blob, &meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func ensureFile(path string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ensure directory for %s: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE, perm)
	if err != nil {
		return fmt.Errorf("ensure file %s: %w", path, err)
	}
	return file.Close()
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat file %s: %w", path, err)
	}
	return info.Size(), nil
}
