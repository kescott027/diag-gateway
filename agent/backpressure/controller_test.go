package backpressure

import "testing"

func TestControllerEvaluateLevels(t *testing.T) {
	c := NewController(Config{})

	tests := []struct {
		name  string
		stats QueueStats
		wantL Level
		wantA Action
	}{
		{"normal", QueueStats{Count: 10, Bytes: 100, MaxCount: 100, MaxBytes: 1000}, LevelNormal, ActionContinue},
		{"warning", QueueStats{Count: 80, Bytes: 100, MaxCount: 100, MaxBytes: 1000}, LevelWarning, ActionSlowDown},
		{"critical", QueueStats{Count: 95, Bytes: 100, MaxCount: 100, MaxBytes: 1000}, LevelCritical, ActionPauseNewStream},
		{"saturated", QueueStats{Count: 101, Bytes: 100, MaxCount: 100, MaxBytes: 1000}, LevelSaturated, ActionBlockEnqueue},
		{"bytes-driven", QueueStats{Count: 10, Bytes: 950, MaxCount: 100, MaxBytes: 1000}, LevelCritical, ActionPauseNewStream},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.Evaluate(tc.stats)
			if err != nil {
				t.Fatalf("evaluate failed: %v", err)
			}
			if got.Level != tc.wantL || got.Action != tc.wantA {
				t.Fatalf("unexpected decision: got=%+v want level=%s action=%s", got, tc.wantL, tc.wantA)
			}
		})
	}
}

func TestControllerRejectsInvalidCapacity(t *testing.T) {
	c := NewController(Config{})
	if _, err := c.Evaluate(QueueStats{Count: 1, Bytes: 1, MaxCount: 0, MaxBytes: 1}); err == nil {
		t.Fatalf("expected invalid capacity error")
	}
}
