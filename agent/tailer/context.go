package tailer

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

const (
	defaultContextMaxReadBytes int64 = 256 * 1024
	maxContextReadBytes        int64 = 8 * 1024 * 1024
)

// TailContext contains bounded last-line context for new file discovery.
type TailContext struct {
	Lines     []string `json:"lines"`
	Truncated bool     `json:"truncated"`
}

// ReadTailContext returns the last N complete lines from a file using bounded reads.
func ReadTailContext(path string, lastNLines int, maxReadBytes int64) (TailContext, error) {
	if path == "" {
		return TailContext{}, fmt.Errorf("path is required")
	}
	if lastNLines <= 0 {
		return TailContext{Lines: []string{}}, nil
	}

	if maxReadBytes <= 0 {
		maxReadBytes = defaultContextMaxReadBytes
	}
	if maxReadBytes > maxContextReadBytes {
		maxReadBytes = maxContextReadBytes
	}

	f, err := os.Open(path)
	if err != nil {
		return TailContext{}, fmt.Errorf("open context path: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return TailContext{}, fmt.Errorf("stat context path: %w", err)
	}
	size := info.Size()
	if size == 0 {
		return TailContext{Lines: []string{}}, nil
	}

	start := int64(0)
	if size > maxReadBytes {
		start = size - maxReadBytes
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return TailContext{}, fmt.Errorf("seek context path: %w", err)
	}

	buf := make([]byte, size-start)
	if _, err := io.ReadFull(f, buf); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return TailContext{}, fmt.Errorf("read context path: %w", err)
	}

	truncated := start > 0
	if start > 0 {
		idx := bytes.IndexByte(buf, '\n')
		if idx < 0 {
			return TailContext{Lines: []string{}, Truncated: true}, nil
		}
		buf = buf[idx+1:]
	}

	if len(buf) == 0 {
		return TailContext{Lines: []string{}, Truncated: truncated}, nil
	}

	parts := bytes.Split(buf, []byte{'\n'})
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return TailContext{Lines: []string{}, Truncated: truncated}, nil
	}
	if len(parts) > lastNLines {
		parts = parts[len(parts)-lastNLines:]
	}

	lines := make([]string, 0, len(parts))
	for _, p := range parts {
		lines = append(lines, string(p))
	}
	return TailContext{Lines: lines, Truncated: truncated}, nil
}
