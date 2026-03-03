package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrPayloadLengthMismatch = errors.New("payload length does not match declared length")
	ErrChecksumMismatch      = errors.New("payload checksum mismatch")
)

// ComputeChecksum returns a deterministic SHA-256 hex digest for payload bytes.
func ComputeChecksum(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// ValidateChunkIntegrity verifies declared payload length and checksum.
func ValidateChunkIntegrity(chunk StreamChunk) error {
	if len(chunk.Payload) != chunk.PayloadLength {
		return fmt.Errorf("%w: declared=%d actual=%d", ErrPayloadLengthMismatch, chunk.PayloadLength, len(chunk.Payload))
	}
	expected := ComputeChecksum(chunk.Payload)
	if chunk.Checksum != expected {
		return fmt.Errorf("%w: expected=%s got=%s", ErrChecksumMismatch, expected, chunk.Checksum)
	}
	return nil
}

