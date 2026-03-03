package protocol

import (
	"errors"
	"testing"
)

func TestValidateChunkIntegritySuccess(t *testing.T) {
	payload := []byte("hello")
	chunk := StreamChunk{
		Payload:       payload,
		PayloadLength: len(payload),
		Checksum:      ComputeChecksum(payload),
	}
	if err := ValidateChunkIntegrity(chunk); err != nil {
		t.Fatalf("expected valid chunk, got %v", err)
	}
}

func TestValidateChunkIntegrityLengthMismatch(t *testing.T) {
	payload := []byte("hello")
	chunk := StreamChunk{
		Payload:       payload,
		PayloadLength: len(payload) + 1,
		Checksum:      ComputeChecksum(payload),
	}
	err := ValidateChunkIntegrity(chunk)
	if !errors.Is(err, ErrPayloadLengthMismatch) {
		t.Fatalf("expected payload length mismatch error, got %v", err)
	}
}

func TestValidateChunkIntegrityChecksumMismatch(t *testing.T) {
	payload := []byte("hello")
	chunk := StreamChunk{
		Payload:       payload,
		PayloadLength: len(payload),
		Checksum:      "deadbeef",
	}
	err := ValidateChunkIntegrity(chunk)
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}

