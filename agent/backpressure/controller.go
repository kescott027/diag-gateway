package backpressure

import (
	"fmt"
)

type Level string

const (
	LevelNormal    Level = "normal"
	LevelWarning   Level = "warning"
	LevelCritical  Level = "critical"
	LevelSaturated Level = "saturated"
)

type Action string

const (
	ActionContinue       Action = "continue"
	ActionSlowDown       Action = "slow_down"
	ActionPauseNewStream Action = "pause_new_streams"
	ActionBlockEnqueue   Action = "block_enqueue"
)

// QueueStats captures spool occupancy and configured limits.
type QueueStats struct {
	Count    int
	Bytes    int64
	MaxCount int
	MaxBytes int64
}

// Decision captures deterministic pressure state and action.
type Decision struct {
	Level       Level
	Action      Action
	Utilization float64
	Reason      string
}

// Config controls occupancy thresholds.
type Config struct {
	WarningThreshold   float64
	CriticalThreshold  float64
	SaturatedThreshold float64
}

func (c Config) withDefaults() Config {
	out := c
	if out.WarningThreshold <= 0 {
		out.WarningThreshold = 0.75
	}
	if out.CriticalThreshold <= 0 {
		out.CriticalThreshold = 0.90
	}
	if out.SaturatedThreshold <= 0 {
		out.SaturatedThreshold = 1.00
	}
	return out
}

// Controller computes bounded backpressure decisions from queue occupancy.
type Controller struct {
	cfg Config
}

func NewController(cfg Config) *Controller {
	return &Controller{cfg: cfg.withDefaults()}
}

func (c *Controller) Evaluate(stats QueueStats) (Decision, error) {
	if stats.MaxCount <= 0 || stats.MaxBytes <= 0 {
		return Decision{}, fmt.Errorf("invalid queue capacity")
	}
	countU := float64(stats.Count) / float64(stats.MaxCount)
	bytesU := float64(stats.Bytes) / float64(stats.MaxBytes)
	u := max(countU, bytesU)

	switch {
	case u >= c.cfg.SaturatedThreshold:
		return Decision{Level: LevelSaturated, Action: ActionBlockEnqueue, Utilization: u, Reason: "queue capacity reached"}, nil
	case u >= c.cfg.CriticalThreshold:
		return Decision{Level: LevelCritical, Action: ActionPauseNewStream, Utilization: u, Reason: "queue near saturation"}, nil
	case u >= c.cfg.WarningThreshold:
		return Decision{Level: LevelWarning, Action: ActionSlowDown, Utilization: u, Reason: "queue pressure rising"}, nil
	default:
		return Decision{Level: LevelNormal, Action: ActionContinue, Utilization: u, Reason: "queue healthy"}, nil
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
