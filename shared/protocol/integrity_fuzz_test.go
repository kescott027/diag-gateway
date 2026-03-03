package protocol

import "testing"

func FuzzValidateChunkIntegrity(f *testing.F) {
	f.Add([]byte("hello"), int8(0), false)
	f.Add([]byte(""), int8(0), false)
	f.Add([]byte("abc"), int8(1), false)
	f.Add([]byte("abc"), int8(0), true)

	f.Fuzz(func(t *testing.T, payload []byte, lengthDelta int8, corruptChecksum bool) {
		if len(payload) > 4096 {
			payload = payload[:4096]
		}

		declared := len(payload) + int(lengthDelta%4)
		if declared < 0 {
			declared = 0
		}
		checksum := ComputeChecksum(payload)
		if corruptChecksum {
			if checksum == "" {
				checksum = "x"
			} else {
				last := checksum[len(checksum)-1]
				replacement := byte('0')
				if last == '0' {
					replacement = '1'
				}
				checksum = checksum[:len(checksum)-1] + string(replacement)
			}
		}

		chunk := StreamChunk{
			Payload:       payload,
			PayloadLength: declared,
			Checksum:      checksum,
		}
		err := ValidateChunkIntegrity(chunk)
		shouldPass := declared == len(payload) && !corruptChecksum
		if shouldPass && err != nil {
			t.Fatalf("expected valid chunk, got %v", err)
		}
		if !shouldPass && err == nil {
			t.Fatalf("expected integrity failure")
		}
	})
}
