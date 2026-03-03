package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Mode specifies payload compression behavior.
type Mode string

const (
	ModeNone Mode = "none"
	ModeGzip Mode = "gzip"
	ModeZstd Mode = "zstd"
)

var supportedModes = []Mode{ModeNone, ModeGzip, ModeZstd}

func ParseMode(value string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(value))) {
	case ModeNone:
		return ModeNone, nil
	case ModeGzip:
		return ModeGzip, nil
	case ModeZstd:
		return ModeZstd, nil
	default:
		return "", fmt.Errorf("unsupported compression mode %q", value)
	}
}

func SupportedModes() []Mode {
	out := make([]Mode, len(supportedModes))
	copy(out, supportedModes)
	return out
}

func Compress(mode Mode, payload []byte) ([]byte, error) {
	switch mode {
	case ModeNone:
		out := make([]byte, len(payload))
		copy(out, payload)
		return out, nil
	case ModeGzip:
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(payload); err != nil {
			return nil, fmt.Errorf("gzip compress: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("gzip close: %w", err)
		}
		return buf.Bytes(), nil
	case ModeZstd:
		var buf bytes.Buffer
		w, err := zstd.NewWriter(&buf)
		if err != nil {
			return nil, fmt.Errorf("zstd writer: %w", err)
		}
		if _, err := w.Write(payload); err != nil {
			return nil, fmt.Errorf("zstd compress: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("zstd close: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported compression mode %q", mode)
	}
}

func Decompress(mode Mode, payload []byte) ([]byte, error) {
	switch mode {
	case ModeNone:
		out := make([]byte, len(payload))
		copy(out, payload)
		return out, nil
	case ModeGzip:
		r, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		decoded, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("gzip decompress: %w", err)
		}
		return decoded, nil
	case ModeZstd:
		r, err := zstd.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("zstd reader: %w", err)
		}
		defer r.Close()
		decoded, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("zstd decompress: %w", err)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unsupported compression mode %q", mode)
	}
}

// Negotiate chooses the first mode supported by both ends.
func Negotiate(preferred []Mode, peerSupported []Mode) Mode {
	peer := make(map[Mode]struct{}, len(peerSupported))
	for _, m := range peerSupported {
		peer[m] = struct{}{}
	}
	for _, m := range preferred {
		if _, ok := peer[m]; ok {
			return m
		}
	}
	return ModeNone
}
