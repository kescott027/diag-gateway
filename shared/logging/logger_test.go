package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestWithCorrelationIDRoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "cid-123")
	got, ok := CorrelationIDFromContext(ctx)
	if !ok || got != "cid-123" {
		t.Fatalf("unexpected correlation id: %q ok=%v", got, ok)
	}
}

func TestLoggerEmitsStructuredEntry(t *testing.T) {
	var buf bytes.Buffer
	l := New("collector.ingest", &buf)
	l.now = func() time.Time {
		return time.Date(2026, 3, 3, 12, 30, 0, 0, time.UTC)
	}

	ctx := WithCorrelationID(context.Background(), "cid-xyz")
	if err := l.Info(ctx, "chunk accepted", map[string]any{
		"source_id": "source-1",
		"stream_id": "stream-1",
		"offset":    42,
	}); err != nil {
		t.Fatalf("log info failed: %v", err)
	}

	var out Entry
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out); err != nil {
		t.Fatalf("unmarshal log output failed: %v", err)
	}
	if out.Component != "collector.ingest" || out.Level != "info" {
		t.Fatalf("unexpected component/level: %+v", out)
	}
	if out.CorrelationID != "cid-xyz" {
		t.Fatalf("unexpected correlation id: %s", out.CorrelationID)
	}
	if got, ok := out.Fields["source_id"].(string); !ok || got != "source-1" {
		t.Fatalf("missing source_id field: %+v", out.Fields)
	}
}

func TestLoggerGeneratesCorrelationIDWhenMissing(t *testing.T) {
	var buf bytes.Buffer
	l := New("agent.tailer", &buf)
	l.now = func() time.Time {
		return time.Date(2026, 3, 3, 12, 31, 0, 0, time.UTC)
	}

	if err := l.Warn(context.Background(), "no context id", nil); err != nil {
		t.Fatalf("warn failed: %v", err)
	}

	var out Entry
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.CorrelationID == "" {
		t.Fatalf("expected generated correlation id")
	}
	if out.Level != "warn" {
		t.Fatalf("unexpected level: %s", out.Level)
	}
}

