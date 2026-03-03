package tailer

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kescott027/diag-gateway/agent/fileid"
	"github.com/kescott027/diag-gateway/agent/rotation"
)

const (
	defaultPollInterval = 500 * time.Millisecond
	defaultChunkSize    = 64 * 1024
)

// Config defines deterministic tailing behavior for a single file.
type Config struct {
	SourceID      string
	FileKey       string
	Path          string
	PollInterval  time.Duration
	ChunkSize     int
	MaxFileSizeMB int
}

// Chunk is emitted for each append payload segment.
type Chunk struct {
	SourceID           string            `json:"source_id"`
	FileKey            string            `json:"file_key"`
	Path               string            `json:"path"`
	Offset             int64             `json:"offset"`
	Payload            []byte            `json:"payload"`
	FileIdentity       string            `json:"file_identity"`
	IdentityConfidence fileid.Confidence `json:"file_identity_confidence"`
	StreamRestart      bool              `json:"stream_restart"`
	RestartReason      string            `json:"restart_reason,omitempty"`
}

// PollResult summarizes one polling cycle.
type PollResult struct {
	BytesRead  int64           `json:"bytes_read"`
	Chunks     int             `json:"chunks"`
	Restarted  bool            `json:"restarted"`
	Action     rotation.Action `json:"action"`
	Skipped    bool            `json:"skipped"`
	SkipReason string          `json:"skip_reason,omitempty"`
}

// ChunkHandler receives emitted chunks. Returning error stops the poll cycle.
type ChunkHandler func(chunk Chunk) error

// Tailer streams appended file data in bounded chunks.
type Tailer struct {
	cfg     Config
	store   rotation.CursorStore
	rotator *rotation.Manager
	handle  ChunkHandler
}

// NewTailer constructs a deterministic polling tailer.
func NewTailer(cfg Config, store rotation.CursorStore, handler ChunkHandler) (*Tailer, error) {
	if cfg.SourceID == "" {
		return nil, fmt.Errorf("source id is required")
	}
	if cfg.FileKey == "" {
		return nil, fmt.Errorf("file key is required")
	}
	if cfg.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = defaultChunkSize
	}
	if cfg.MaxFileSizeMB < 0 {
		return nil, fmt.Errorf("max file size must be >= 0")
	}
	if handler == nil {
		return nil, fmt.Errorf("chunk handler is required")
	}
	if store == nil {
		return nil, fmt.Errorf("cursor store is required")
	}

	return &Tailer{
		cfg:     cfg,
		store:   store,
		rotator: rotation.NewManager(store),
		handle:  handler,
	}, nil
}

// PollOnce performs one append scan and emits newly observed bytes.
func (t *Tailer) PollOnce() (PollResult, error) {
	info, err := os.Stat(t.cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return PollResult{}, nil
		}
		return PollResult{}, fmt.Errorf("stat path: %w", err)
	}
	if t.cfg.MaxFileSizeMB > 0 {
		maxSizeBytes := int64(t.cfg.MaxFileSizeMB) * 1024 * 1024
		if info.Size() > maxSizeBytes {
			return PollResult{
				Skipped:    true,
				SkipReason: fmt.Sprintf("file size %d exceeds max %d bytes", info.Size(), maxSizeBytes),
			}, nil
		}
	}

	identity, err := fileid.Identify(t.cfg.Path, info)
	if err != nil {
		return PollResult{}, fmt.Errorf("identify file: %w", err)
	}

	decision, err := t.rotator.EvaluateAndAssign(t.cfg.SourceID, t.cfg.FileKey, identity)
	if err != nil {
		return PollResult{}, fmt.Errorf("evaluate rotation/truncation: %w", err)
	}

	file, err := os.Open(t.cfg.Path)
	if err != nil {
		return PollResult{}, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	offset := decision.StartOffset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return PollResult{}, fmt.Errorf("seek to offset %d: %w", offset, err)
	}

	buf := make([]byte, t.cfg.ChunkSize)
	result := PollResult{Restarted: decision.NewStream, Action: decision.Action}
	identityToken := identity.Encode()

	firstChunk := true
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			payload := make([]byte, n)
			copy(payload, buf[:n])

			chunk := Chunk{
				SourceID:           t.cfg.SourceID,
				FileKey:            t.cfg.FileKey,
				Path:               t.cfg.Path,
				Offset:             offset,
				Payload:            payload,
				FileIdentity:       identityToken,
				IdentityConfidence: identity.Confidence,
				StreamRestart:      decision.NewStream && firstChunk,
				RestartReason:      decision.Reason,
			}
			if err := t.handle(chunk); err != nil {
				return result, fmt.Errorf("handle chunk: %w", err)
			}

			offset += int64(n)
			result.BytesRead += int64(n)
			result.Chunks++
			firstChunk = false

			if err := t.store.Set(t.cfg.SourceID, t.cfg.FileKey, identityToken, offset); err != nil {
				return result, fmt.Errorf("persist cursor offset: %w", err)
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return result, fmt.Errorf("read file: %w", readErr)
		}
	}

	return result, nil
}

// Run executes polling until context cancellation.
func (t *Tailer) Run(ctx context.Context) error {
	ticker := time.NewTicker(t.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := t.PollOnce(); err != nil {
				return err
			}
		}
	}
}
