package reassembly

import "testing"

func FuzzChunkProcessorDuplicateDeterminism(f *testing.F) {
	f.Add([]byte("abc"), byte(1))
	f.Add([]byte(""), byte(1))

	f.Fuzz(func(t *testing.T, payload []byte, flip byte) {
		if len(payload) > 4096 {
			payload = payload[:4096]
		}
		if flip == 0 {
			flip = 1
		}

		tmp := t.TempDir()
		reassembler := NewReassembler(tmp)
		if err := reassembler.OpenStream("source-1", "stream-fuzz", "app.log"); err != nil {
			t.Fatalf("open stream failed: %v", err)
		}

		processor := NewChunkProcessor(reassembler, 256)
		first, err := processor.ProcessChunk("source-1", "stream-fuzz", 1, 0, payload)
		if err != nil {
			t.Fatalf("first process failed: %v", err)
		}
		if first.Duplicate {
			t.Fatalf("first process must not be duplicate")
		}

		dup, err := processor.ProcessChunk("source-1", "stream-fuzz", 1, 0, payload)
		if err != nil {
			t.Fatalf("duplicate process failed: %v", err)
		}
		if !dup.Duplicate {
			t.Fatalf("expected duplicate replay ack")
		}
		if dup.CurrentOffset != int64(len(payload)) {
			t.Fatalf("unexpected current offset: %d", dup.CurrentOffset)
		}

		if len(payload) == 0 {
			return
		}

		mutated := append([]byte(nil), payload...)
		mutated[0] ^= flip
		if mutated[0] == payload[0] {
			mutated[0] ^= 1
		}
		if _, err := processor.ProcessChunk("source-1", "stream-fuzz", 1, 0, mutated); err == nil {
			t.Fatalf("expected sequence conflict for mutated duplicate payload")
		}
	})
}

func FuzzChunkProcessorOffsetContinuity(f *testing.F) {
	f.Add([]byte("hello"), int64(5))
	f.Add([]byte("x"), int64(0))

	f.Fuzz(func(t *testing.T, payload []byte, secondOffset int64) {
		if len(payload) > 2048 {
			payload = payload[:2048]
		}
		if secondOffset < 0 {
			secondOffset = -secondOffset
		}
		secondOffset = secondOffset % 4096

		tmp := t.TempDir()
		reassembler := NewReassembler(tmp)
		if err := reassembler.OpenStream("source-1", "stream-offset", "svc.log"); err != nil {
			t.Fatalf("open stream failed: %v", err)
		}
		processor := NewChunkProcessor(reassembler, 256)

		if _, err := processor.ProcessChunk("source-1", "stream-offset", 1, 0, payload); err != nil {
			t.Fatalf("first process failed: %v", err)
		}

		_, err := processor.ProcessChunk("source-1", "stream-offset", 2, secondOffset, []byte("z"))
		expectedOffset := int64(len(payload))
		if secondOffset == expectedOffset {
			if err != nil {
				t.Fatalf("expected valid second chunk at offset %d, got err %v", secondOffset, err)
			}
			return
		}
		if err == nil {
			t.Fatalf("expected offset mismatch for offset %d (expected %d)", secondOffset, expectedOffset)
		}
	})
}
