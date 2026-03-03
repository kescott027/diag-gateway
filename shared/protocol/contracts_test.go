package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventEnvelopeJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 3, 11, 20, 0, 0, time.UTC)
	env := EventEnvelope{
		EventTime:           now,
		IngestTime:          now.Add(time.Second),
		TenantID:            "tenant-1",
		SourceID:            "source-1",
		SourceType:          "app",
		RawPayload:          "hello world",
		Fingerprint:         "fp-1",
		FingerprintVersion:  "1",
		StructureClass:      StructureStructured,
		StructureConfidence: 0.98,
		SamplingFlag:        SamplingFull,
		PriorityFlag:        PriorityHigh,
	}

	blob, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out EventEnvelope
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if out.TenantID != env.TenantID {
		t.Fatalf("tenant_id mismatch: got %q want %q", out.TenantID, env.TenantID)
	}
	if out.FingerprintVersion != "1" {
		t.Fatalf("fingerprint_version missing")
	}
	if out.SamplingFlag != SamplingFull {
		t.Fatalf("sampling_flag mismatch")
	}
	if out.StructureConfidence <= 0 {
		t.Fatalf("structure_confidence should be retained")
	}
}

func TestSignalSchemaContainsRequiredFields(t *testing.T) {
	now := time.Date(2026, 3, 3, 11, 20, 0, 0, time.UTC)
	s := Signal{
		SignalID:    "sig-1",
		TenantID:    "tenant-1",
		SourceScope: []string{"source-1"},
		SignalType:  "anomaly",
		Score:       0.9,
		Confidence:  0.8,
		Window: SignalWindow{
			Start:           now,
			End:             now.Add(5 * time.Minute),
			DurationSeconds: 300,
		},
		ContributingFingerprints: []string{"fp-1"},
		Deviation:                SignalDeviation{Metric: "volume_zscore", Value: 3.2},
		SamplingImpact:           "low",
		Explainability:           "volume increase over baseline",
	}

	blob, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(blob, &m); err != nil {
		t.Fatalf("unmarshal into map failed: %v", err)
	}

	required := []string{"signal_id", "score", "confidence", "window", "contributing_fingerprints", "sampling_impact"}
	for _, key := range required {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing required field %q", key)
		}
	}
}
