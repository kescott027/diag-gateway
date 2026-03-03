package rotation

import (
	"testing"

	"github.com/kescott027/diag-gateway/agent/cursor"
	"github.com/kescott027/diag-gateway/agent/fileid"
)

type memoryStore struct {
	data map[string]cursor.Cursor
}

func newMemoryStore() *memoryStore {
	return &memoryStore{data: make(map[string]cursor.Cursor)}
}

func (m *memoryStore) Get(sourceID, fileKey string) (cursor.Cursor, bool) {
	cur, ok := m.data[sourceID+"|"+fileKey]
	return cur, ok
}

func (m *memoryStore) Set(sourceID, fileKey, fileIdentity string, offset int64) error {
	m.data[sourceID+"|"+fileKey] = cursor.Cursor{
		SourceID:     sourceID,
		FileKey:      fileKey,
		FileIdentity: fileIdentity,
		Offset:       offset,
	}
	return nil
}

func TestEvaluateAndAssignStartForNewCursor(t *testing.T) {
	store := newMemoryStore()
	mgr := NewManager(store)
	observed := fileid.Identity{
		Version:         fileid.Version,
		Method:          fileid.MethodNativeInodeDev,
		Confidence:      fileid.ConfidenceStrong,
		Path:            "/tmp/app.log",
		Device:          1,
		Inode:           10,
		Size:            0,
		ModTimeUnixNano: 1,
	}

	decision, err := mgr.EvaluateAndAssign("source-1", "app.log", observed)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != ActionStart {
		t.Fatalf("expected start action, got %s", decision.Action)
	}
	if decision.StartOffset != 0 || !decision.NewStream {
		t.Fatalf("unexpected start decision: %+v", decision)
	}

	cur, ok := store.Get("source-1", "app.log")
	if !ok || cur.Offset != 0 {
		t.Fatalf("expected cursor persisted at zero: %+v", cur)
	}
}

func TestEvaluateResumeForFallbackAppend(t *testing.T) {
	previous := fileid.Identity{
		Version:         fileid.Version,
		Method:          fileid.MethodPathSizeMTime,
		Confidence:      fileid.ConfidenceFallback,
		Path:            "/tmp/service.log",
		Size:            100,
		ModTimeUnixNano: 1000,
	}
	current := previous
	current.Size = 150
	current.ModTimeUnixNano = 2000

	existing := cursor.Cursor{
		SourceID:     "source-1",
		FileKey:      "service.log",
		FileIdentity: previous.Encode(),
		Offset:       80,
	}
	decision, err := Evaluate(current, existing, true)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != ActionResume {
		t.Fatalf("expected resume, got %s", decision.Action)
	}
	if decision.StartOffset != 80 || decision.NewStream {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestEvaluateDetectsTruncation(t *testing.T) {
	id := fileid.Identity{
		Version:         fileid.Version,
		Method:          fileid.MethodNativeInodeDev,
		Confidence:      fileid.ConfidenceStrong,
		Path:            "/tmp/app.log",
		Device:          1,
		Inode:           10,
		Size:            20,
		ModTimeUnixNano: 2,
	}
	existing := cursor.Cursor{
		SourceID:     "source-1",
		FileKey:      "app.log",
		FileIdentity: id.Encode(),
		Offset:       30,
	}

	decision, err := Evaluate(id, existing, true)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != ActionReopenTruncated {
		t.Fatalf("expected truncation reopen, got %s", decision.Action)
	}
	if decision.StartOffset != 0 || !decision.NewStream {
		t.Fatalf("unexpected truncation decision: %+v", decision)
	}
}

func TestEvaluateDetectsRotationFromNativeIdentityChange(t *testing.T) {
	prev := fileid.Identity{
		Version:         fileid.Version,
		Method:          fileid.MethodNativeInodeDev,
		Confidence:      fileid.ConfidenceStrong,
		Path:            "/tmp/api.log",
		Device:          1,
		Inode:           10,
		Size:            100,
		ModTimeUnixNano: 1,
	}
	current := prev
	current.Inode = 11
	current.Size = 10

	existing := cursor.Cursor{
		SourceID:     "source-1",
		FileKey:      "api.log",
		FileIdentity: prev.Encode(),
		Offset:       100,
	}

	decision, err := Evaluate(current, existing, true)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != ActionReopenRotated {
		t.Fatalf("expected rotation reopen, got %s", decision.Action)
	}
}

func TestEvaluateUnreadableIdentityIsConservative(t *testing.T) {
	current := fileid.Identity{
		Version:         fileid.Version,
		Method:          fileid.MethodPathSizeMTime,
		Confidence:      fileid.ConfidenceFallback,
		Path:            "/tmp/bad.log",
		Size:            10,
		ModTimeUnixNano: 1,
	}
	existing := cursor.Cursor{
		SourceID:     "source-1",
		FileKey:      "bad.log",
		FileIdentity: "unparseable-legacy-format",
		Offset:       5,
	}

	decision, err := Evaluate(current, existing, true)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != ActionReopenRotated {
		t.Fatalf("expected conservative reopen, got %s", decision.Action)
	}
}

