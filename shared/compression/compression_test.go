package compression

import (
	"bytes"
	"testing"
)

func TestRoundTripModes(t *testing.T) {
	payload := bytes.Repeat([]byte("diag-gateway compression payload\n"), 64)
	modes := []Mode{ModeNone, ModeGzip, ModeZstd}

	for _, mode := range modes {
		encoded, err := Compress(mode, payload)
		if err != nil {
			t.Fatalf("compress failed for mode %s: %v", mode, err)
		}
		decoded, err := Decompress(mode, encoded)
		if err != nil {
			t.Fatalf("decompress failed for mode %s: %v", mode, err)
		}
		if !bytes.Equal(decoded, payload) {
			t.Fatalf("roundtrip mismatch for mode %s", mode)
		}
	}
}

func TestInvalidModes(t *testing.T) {
	if _, err := ParseMode("brotli"); err == nil {
		t.Fatalf("expected parse error for unsupported mode")
	}
	if _, err := Compress(Mode("brotli"), []byte("x")); err == nil {
		t.Fatalf("expected compress error for unsupported mode")
	}
	if _, err := Decompress(Mode("brotli"), []byte("x")); err == nil {
		t.Fatalf("expected decompress error for unsupported mode")
	}
}

func TestNegotiate(t *testing.T) {
	chosen := Negotiate([]Mode{ModeZstd, ModeGzip}, []Mode{ModeGzip, ModeNone})
	if chosen != ModeGzip {
		t.Fatalf("expected gzip negotiation, got %s", chosen)
	}
	fallback := Negotiate([]Mode{ModeZstd}, []Mode{ModeGzip})
	if fallback != ModeNone {
		t.Fatalf("expected none fallback, got %s", fallback)
	}
}
