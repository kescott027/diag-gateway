package watcher

import (
	"testing"
	"time"
)

func TestAdaptivePolicyIntervalTiers(t *testing.T) {
	policy := AdaptivePollingPolicy{
		HotInterval:  200 * time.Millisecond,
		WarmInterval: 800 * time.Millisecond,
		ColdInterval: 2 * time.Second,
		WarmAfter:    10 * time.Second,
		ColdAfter:    30 * time.Second,
	}

	now := time.Unix(1000, 0)
	if got := policy.Interval(now, time.Time{}); got != 200*time.Millisecond {
		t.Fatalf("expected hot interval for zero activity, got %s", got)
	}
	if got := policy.Interval(now, now.Add(-5*time.Second)); got != 200*time.Millisecond {
		t.Fatalf("expected hot interval, got %s", got)
	}
	if got := policy.Interval(now, now.Add(-15*time.Second)); got != 800*time.Millisecond {
		t.Fatalf("expected warm interval, got %s", got)
	}
	if got := policy.Interval(now, now.Add(-45*time.Second)); got != 2*time.Second {
		t.Fatalf("expected cold interval, got %s", got)
	}
}

func TestAdaptivePolicyNormalizeBounds(t *testing.T) {
	policy := AdaptivePollingPolicy{
		HotInterval:  500 * time.Millisecond,
		WarmInterval: 100 * time.Millisecond,
		ColdInterval: 200 * time.Millisecond,
		WarmAfter:    20 * time.Second,
		ColdAfter:    5 * time.Second,
	}

	n := policy.normalize()
	if n.WarmInterval != n.HotInterval {
		t.Fatalf("expected warm interval to be clamped to hot interval")
	}
	if n.ColdInterval != n.WarmInterval {
		t.Fatalf("expected cold interval to be clamped to warm interval")
	}
	if n.ColdAfter != n.WarmAfter {
		t.Fatalf("expected cold-after to be clamped to warm-after")
	}
	if got := policy.MinInterval(); got != 500*time.Millisecond {
		t.Fatalf("expected min interval to equal hot interval, got %s", got)
	}
}
