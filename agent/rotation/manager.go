package rotation

import (
	"errors"
	"fmt"

	"github.com/kescott027/diag-gateway/agent/cursor"
	"github.com/kescott027/diag-gateway/agent/fileid"
)

var ErrNegativeObservedSize = errors.New("observed file size cannot be negative")

type Action string

const (
	ActionStart           Action = "start"
	ActionResume          Action = "resume"
	ActionReopenRotated   Action = "reopen_rotated"
	ActionReopenTruncated Action = "reopen_truncated"
)

// Decision captures deterministic cursor action for file open/reopen.
type Decision struct {
	Action      Action            `json:"action"`
	StartOffset int64             `json:"start_offset"`
	NewStream   bool              `json:"new_stream"`
	Reason      string            `json:"reason"`
	Confidence  fileid.Confidence `json:"confidence"`
}

// CursorStore captures cursor persistence operations needed by Manager.
type CursorStore interface {
	Get(sourceID, fileKey string) (cursor.Cursor, bool)
	Set(sourceID, fileKey, fileIdentity string, offset int64) error
}

// Manager evaluates and persists rotation/truncation-safe cursor assignment.
type Manager struct {
	store CursorStore
}

func NewManager(store CursorStore) *Manager {
	return &Manager{store: store}
}

// EvaluateAndAssign returns the safe starting offset and updates persisted cursor assignment.
func (m *Manager) EvaluateAndAssign(sourceID, fileKey string, observed fileid.Identity) (Decision, error) {
	if m.store == nil {
		return Decision{}, fmt.Errorf("cursor store is required")
	}

	existing, ok := m.store.Get(sourceID, fileKey)
	decision, err := Evaluate(observed, existing, ok)
	if err != nil {
		return Decision{}, err
	}

	if err := m.store.Set(sourceID, fileKey, observed.Encode(), decision.StartOffset); err != nil {
		return Decision{}, fmt.Errorf("persist cursor assignment: %w", err)
	}
	return decision, nil
}

// Evaluate computes deterministic file continuity behavior without persistence side effects.
func Evaluate(observed fileid.Identity, existing cursor.Cursor, hasCursor bool) (Decision, error) {
	if observed.Size < 0 {
		return Decision{}, ErrNegativeObservedSize
	}

	base := Decision{
		StartOffset: 0,
		NewStream:   true,
		Confidence:  observed.Confidence,
	}

	if !hasCursor {
		base.Action = ActionStart
		base.Reason = "new file watch with no existing cursor"
		return base, nil
	}

	previousOffset := existing.Offset
	if previousOffset < 0 {
		previousOffset = 0
	}

	prevIdentity, err := fileid.Parse(existing.FileIdentity)
	if err != nil {
		base.Action = ActionReopenRotated
		base.Reason = "stored file identity unreadable; reopening from offset 0"
		return base, nil
	}

	if !prevIdentity.SameEntity(observed) {
		base.Action = ActionReopenRotated
		base.Reason = "file identity changed"
		return base, nil
	}

	if observed.Size < previousOffset {
		base.Action = ActionReopenTruncated
		base.Reason = "file size rolled back below stored cursor offset"
		return base, nil
	}

	return Decision{
		Action:      ActionResume,
		StartOffset: previousOffset,
		NewStream:   false,
		Reason:      "continuing existing file stream",
		Confidence:  observed.Confidence,
	}, nil
}

