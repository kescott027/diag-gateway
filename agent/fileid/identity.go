package fileid

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrNilFileInfo = errors.New("nil file info")

// Version identifies the serialized identity schema version.
const Version = "1"

// Method indicates how file identity was derived.
type Method string

const (
	MethodNativeInodeDev Method = "native_inode_dev"
	MethodPathSizeMTime  Method = "path_size_mtime"
)

// Confidence captures how stable the identity is expected to be.
type Confidence string

const (
	ConfidenceStrong   Confidence = "strong"
	ConfidenceFallback Confidence = "fallback"
)

// Identity models cross-platform file identity with fallback metadata.
type Identity struct {
	Version         string     `json:"version"`
	Method          Method     `json:"method"`
	Confidence      Confidence `json:"confidence"`
	Path            string     `json:"path"`
	Device          uint64     `json:"device,omitempty"`
	Inode           uint64     `json:"inode,omitempty"`
	Size            int64      `json:"size"`
	ModTimeUnixNano int64      `json:"mod_time_unix_nano"`
}

// IdentifyPath computes identity from a filesystem path.
func IdentifyPath(path string) (Identity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Identity{}, fmt.Errorf("stat path: %w", err)
	}
	return Identify(path, info)
}

// Identify computes identity from path and file metadata.
func Identify(path string, info os.FileInfo) (Identity, error) {
	if info == nil {
		return Identity{}, ErrNilFileInfo
	}

	normalizedPath := normalizePath(path)
	size := info.Size()
	mod := info.ModTime().UTC().UnixNano()

	if dev, ino, ok := extractNativeDeviceInode(info); ok {
		return Identity{
			Version:         Version,
			Method:          MethodNativeInodeDev,
			Confidence:      ConfidenceStrong,
			Path:            normalizedPath,
			Device:          dev,
			Inode:           ino,
			Size:            size,
			ModTimeUnixNano: mod,
		}, nil
	}

	return Identity{
		Version:         Version,
		Method:          MethodPathSizeMTime,
		Confidence:      ConfidenceFallback,
		Path:            normalizedPath,
		Size:            size,
		ModTimeUnixNano: mod,
	}, nil
}

// Encode serializes identity for cursor persistence.
func (i Identity) Encode() string {
	if i.Version == "" {
		i.Version = Version
	}
	blob, err := json.Marshal(i)
	if err != nil {
		// The struct fields are marshal-safe; this fallback keeps behavior deterministic.
		return fmt.Sprintf(`{"version":"%s","method":"%s","confidence":"%s","path":"%s","size":%d,"mod_time_unix_nano":%d}`,
			i.Version, i.Method, i.Confidence, i.Path, i.Size, i.ModTimeUnixNano)
	}
	return string(blob)
}

// Parse decodes persisted identity representation.
func Parse(raw string) (Identity, error) {
	var out Identity
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		if out.Version == "" {
			out.Version = Version
		}
		out.Path = normalizePath(out.Path)
		return out, nil
	}

	legacy := strings.TrimSpace(raw)
	if strings.HasPrefix(legacy, "inode:") {
		var inode uint64
		if _, err := fmt.Sscanf(legacy, "inode:%d", &inode); err == nil {
			return Identity{
				Version:    Version,
				Method:     MethodNativeInodeDev,
				Confidence: ConfidenceFallback,
				Inode:      inode,
			}, nil
		}
	}

	return Identity{}, fmt.Errorf("parse file identity")
}

// SameEntity determines if two observations refer to the same underlying file.
func (i Identity) SameEntity(other Identity) bool {
	if i.Method == MethodNativeInodeDev && other.Method == MethodNativeInodeDev {
		return i.Device == other.Device && i.Inode == other.Inode
	}

	if i.Path == "" || other.Path == "" {
		return false
	}

	return samePath(i.Path, other.Path)
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return clean
	}
	return abs
}
