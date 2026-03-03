package observability

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// HealthStatus captures service liveness/readiness summary.
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
	StatusFail     = "fail"
)

// Registry stores lightweight process metrics for /metrics export.
type Registry struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
}

func NewRegistry() *Registry {
	return &Registry{
		counters: make(map[string]float64),
		gauges:   make(map[string]float64),
	}
}

func (r *Registry) IncCounter(name string, delta float64) {
	if delta <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[sanitizeMetricName(name)] += delta
}

func (r *Registry) SetGauge(name string, value float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[sanitizeMetricName(name)] = value
}

func (r *Registry) renderPrometheus() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	lines := make([]string, 0, len(r.counters)+len(r.gauges))
	counterKeys := make([]string, 0, len(r.counters))
	for k := range r.counters {
		counterKeys = append(counterKeys, k)
	}
	sort.Strings(counterKeys)
	for _, k := range counterKeys {
		lines = append(lines, fmt.Sprintf("diag_gateway_%s_total %g", k, r.counters[k]))
	}

	gaugeKeys := make([]string, 0, len(r.gauges))
	for k := range r.gauges {
		gaugeKeys = append(gaugeKeys, k)
	}
	sort.Strings(gaugeKeys)
	for _, k := range gaugeKeys {
		lines = append(lines, fmt.Sprintf("diag_gateway_%s %g", k, r.gauges[k]))
	}

	return strings.Join(lines, "\n") + "\n"
}

// NewHTTPHandler returns an http.Handler exposing /health and /metrics.
func NewHTTPHandler(registry *Registry, healthFn func() HealthStatus) http.Handler {
	if registry == nil {
		registry = NewRegistry()
	}
	if healthFn == nil {
		healthFn = func() HealthStatus {
			return HealthStatus{
				Status:    StatusOK,
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := healthFn()
		if status.Timestamp == "" {
			status.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if status.Status == "" {
			status.Status = StatusOK
		}

		code := http.StatusOK
		if status.Status == StatusFail {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(status)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(registry.renderPrometheus()))
	})

	return mux
}

func sanitizeMetricName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "unnamed"
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.ToLower(b.String())
}

