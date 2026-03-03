package livetail

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

const (
	defaultMaxBytes = 64 * 1024
	maxAllowedBytes = 1024 * 1024
	defaultMaxLines = 200
	maxAllowedLines = 2000
)

var ErrStreamNotFound = errors.New("stream not found")

// Query defines one bounded live-tail poll request.
type Query struct {
	SourceID string `json:"source_id"`
	StreamID string `json:"stream_id"`
	Offset   int64  `json:"offset"`
	MaxBytes int    `json:"max_bytes"`
	MaxLines int    `json:"max_lines"`
}

// Entry is one line-oriented payload emitted for live-tail polling.
type Entry struct {
	Offset  int64  `json:"offset"`
	Line    string `json:"line"`
	Partial bool   `json:"partial"`
}

// Response is the deterministic output for one bounded poll.
type Response struct {
	SourceID   string  `json:"source_id"`
	StreamID   string  `json:"stream_id"`
	Offset     int64   `json:"offset"`
	NextOffset int64   `json:"next_offset"`
	EOF        bool    `json:"eof"`
	Truncated  bool    `json:"truncated"`
	Entries    []Entry `json:"entries"`
}

// Service provides bounded, cursor-oriented live-tail reads.
type Service struct {
	dataDir string
}

func NewService(dataDir string) *Service {
	if dataDir == "" {
		dataDir = "./collector-data"
	}
	return &Service{dataDir: dataDir}
}

func (s *Service) Poll(query Query) (Response, error) {
	if query.Offset < 0 {
		return Response{}, fmt.Errorf("offset must be >= 0")
	}

	logPath, err := layout.StreamLogPath(s.dataDir, query.SourceID, query.StreamID)
	if err != nil {
		return Response{}, err
	}

	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Response{}, fmt.Errorf("%w: %s/%s", ErrStreamNotFound, query.SourceID, query.StreamID)
		}
		return Response{}, fmt.Errorf("stat stream log: %w", err)
	}

	maxBytes, maxLines := normalizeLimits(query.MaxBytes, query.MaxLines)
	offset := query.Offset
	if offset > info.Size() {
		offset = info.Size()
	}

	buf, readErr := readBounded(logPath, offset, maxBytes)
	if readErr != nil {
		return Response{}, readErr
	}

	entries, consumed, truncated := decodeEntries(buf, offset, maxLines)
	nextOffset := offset + int64(consumed)
	if nextOffset > info.Size() {
		nextOffset = info.Size()
	}
	eof := nextOffset >= info.Size()
	if !eof && consumed >= maxBytes {
		truncated = true
	}

	return Response{
		SourceID:   query.SourceID,
		StreamID:   query.StreamID,
		Offset:     offset,
		NextOffset: nextOffset,
		EOF:        eof,
		Truncated:  truncated,
		Entries:    entries,
	}, nil
}

func normalizeLimits(maxBytes, maxLines int) (int, int) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	if maxBytes > maxAllowedBytes {
		maxBytes = maxAllowedBytes
	}

	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}
	if maxLines > maxAllowedLines {
		maxLines = maxAllowedLines
	}
	return maxBytes, maxLines
}

func readBounded(path string, offset int64, maxBytes int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open stream log: %w", err)
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek stream log: %w", err)
	}

	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read stream log: %w", err)
	}
	return buf[:n], nil
}

func decodeEntries(data []byte, baseOffset int64, maxLines int) ([]Entry, int, bool) {
	if len(data) == 0 {
		return []Entry{}, 0, false
	}

	entries := make([]Entry, 0, min(maxLines, 16))
	start := 0
	lineOffset := baseOffset
	for i, b := range data {
		if b != '\n' {
			continue
		}
		entries = append(entries, Entry{
			Offset:  lineOffset,
			Line:    string(data[start:i]),
			Partial: false,
		})
		if len(entries) >= maxLines {
			return entries, i + 1, i+1 < len(data)
		}
		start = i + 1
		lineOffset = baseOffset + int64(start)
	}

	if start < len(data) && len(entries) < maxLines {
		entries = append(entries, Entry{
			Offset:  lineOffset,
			Line:    string(bytes.TrimRight(data[start:], "\n")),
			Partial: true,
		})
	}

	truncated := len(entries) >= maxLines && len(data) > start
	return entries, len(data), truncated
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
