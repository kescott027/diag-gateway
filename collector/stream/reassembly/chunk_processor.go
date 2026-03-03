package reassembly

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidSequence    = errors.New("invalid sequence")
	ErrOutOfOrderSequence = errors.New("out-of-order sequence")
	ErrSequenceConflict   = errors.New("sequence already processed with different chunk attributes")
)

// ProcessResult describes dedupe and sequencing state after chunk processing.
type ProcessResult struct {
	Duplicate            bool
	NextExpectedSequence int64
	CurrentOffset        int64
}

type sequenceRecord struct {
	offset int64
	length int
	sum    string
}

type sequenceState struct {
	next      int64
	processed map[int64]sequenceRecord
}

// ChunkProcessor enforces idempotent sequence handling over append-only reassembly.
type ChunkProcessor struct {
	reassembler *Reassembler
	maxWindow   int64

	mu     sync.Mutex
	states map[streamKey]*sequenceState
}

func NewChunkProcessor(reassembler *Reassembler, maxWindow int64) *ChunkProcessor {
	if maxWindow <= 0 {
		maxWindow = 2048
	}
	return &ChunkProcessor{
		reassembler: reassembler,
		maxWindow:   maxWindow,
		states:      make(map[streamKey]*sequenceState),
	}
}

// ProcessChunk appends in-order chunks and deduplicates safe retries.
func (p *ChunkProcessor) ProcessChunk(sourceID, streamID string, sequence, offset int64, payload []byte) (ProcessResult, error) {
	if sequence < 1 {
		return ProcessResult{}, ErrInvalidSequence
	}

	sum := checksum(payload)

	key := streamKey{sourceID: sourceID, streamID: streamID}

	p.mu.Lock()
	state, ok := p.states[key]
	if !ok {
		state = &sequenceState{
			next:      1,
			processed: make(map[int64]sequenceRecord),
		}
		p.states[key] = state
	}

	if prev, seen := state.processed[sequence]; seen {
		if prev.offset != offset || prev.length != len(payload) || prev.sum != sum {
			p.mu.Unlock()
			return ProcessResult{}, fmt.Errorf("%w: sequence=%d", ErrSequenceConflict, sequence)
		}
		result := ProcessResult{Duplicate: true, NextExpectedSequence: state.next, CurrentOffset: prev.offset + int64(prev.length)}
		p.mu.Unlock()
		return result, nil
	}

	if sequence != state.next {
		expected := state.next
		p.mu.Unlock()
		return ProcessResult{}, fmt.Errorf("%w: expected=%d got=%d", ErrOutOfOrderSequence, expected, sequence)
	}
	p.mu.Unlock()

	newOffset, err := p.reassembler.AppendChunk(sourceID, streamID, offset, payload)
	if err != nil {
		return ProcessResult{}, err
	}

	p.mu.Lock()
	state = p.states[key]
	state.processed[sequence] = sequenceRecord{offset: offset, length: len(payload), sum: sum}
	state.next++
	p.prune(state)
	next := state.next
	p.mu.Unlock()

	return ProcessResult{Duplicate: false, NextExpectedSequence: next, CurrentOffset: newOffset}, nil
}

func checksum(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (p *ChunkProcessor) prune(state *sequenceState) {
	cutoff := state.next - p.maxWindow
	if cutoff <= 0 {
		return
	}
	for seq := range state.processed {
		if seq < cutoff {
			delete(state.processed, seq)
		}
	}
}
