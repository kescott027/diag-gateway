package watcher

import "time"

// AdaptivePollingPolicy defines activity-based polling intervals.
type AdaptivePollingPolicy struct {
	HotInterval  time.Duration
	WarmInterval time.Duration
	ColdInterval time.Duration

	WarmAfter time.Duration
	ColdAfter time.Duration
}

func DefaultAdaptivePollingPolicy() AdaptivePollingPolicy {
	return AdaptivePollingPolicy{
		HotInterval:  250 * time.Millisecond,
		WarmInterval: time.Second,
		ColdInterval: 5 * time.Second,
		WarmAfter:    15 * time.Second,
		ColdAfter:    2 * time.Minute,
	}
}

func (p AdaptivePollingPolicy) normalize() AdaptivePollingPolicy {
	defaults := DefaultAdaptivePollingPolicy()

	if p.HotInterval <= 0 {
		p.HotInterval = defaults.HotInterval
	}
	if p.WarmInterval <= 0 {
		p.WarmInterval = defaults.WarmInterval
	}
	if p.ColdInterval <= 0 {
		p.ColdInterval = defaults.ColdInterval
	}

	if p.WarmInterval < p.HotInterval {
		p.WarmInterval = p.HotInterval
	}
	if p.ColdInterval < p.WarmInterval {
		p.ColdInterval = p.WarmInterval
	}

	if p.WarmAfter <= 0 {
		p.WarmAfter = defaults.WarmAfter
	}
	if p.ColdAfter <= 0 {
		p.ColdAfter = defaults.ColdAfter
	}
	if p.ColdAfter < p.WarmAfter {
		p.ColdAfter = p.WarmAfter
	}
	return p
}

func (p AdaptivePollingPolicy) MinInterval() time.Duration {
	n := p.normalize()
	return n.HotInterval
}

func (p AdaptivePollingPolicy) Interval(now, lastActivity time.Time) time.Duration {
	n := p.normalize()
	if lastActivity.IsZero() {
		return n.HotInterval
	}

	idle := now.Sub(lastActivity)
	if idle >= n.ColdAfter {
		return n.ColdInterval
	}
	if idle >= n.WarmAfter {
		return n.WarmInterval
	}
	return n.HotInterval
}
