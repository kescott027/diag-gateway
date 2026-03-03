package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthEndpointStatusCodes(t *testing.T) {
	reg := NewRegistry()
	handler := NewHTTPHandler(reg, func() HealthStatus {
		return HealthStatus{
			Status:    StatusFail,
			Timestamp: time.Date(2026, 3, 3, 12, 40, 0, 0, time.UTC).Format(time.RFC3339Nano),
			Checks: map[string]string{
				"db": "down",
			},
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for fail health, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("unexpected content type: %s", got)
	}

	var body HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal health body failed: %v", err)
	}
	if body.Status != StatusFail {
		t.Fatalf("unexpected health status: %s", body.Status)
	}
}

func TestMetricsEndpointPrometheusFormat(t *testing.T) {
	reg := NewRegistry()
	reg.IncCounter("Chunks Accepted", 2)
	reg.SetGauge("queue.depth", 4)

	handler := NewHTTPHandler(reg, nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "diag_gateway_chunks_accepted_total 2") {
		t.Fatalf("missing counter metric: %q", body)
	}
	if !strings.Contains(body, "diag_gateway_queue_depth 4") {
		t.Fatalf("missing gauge metric: %q", body)
	}
}

