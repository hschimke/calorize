package telemetry_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"azule.info/calorize/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetricsEndpointAndRecording(t *testing.T) {
	// Calling InitMetrics multiple times should be safe and idempotent.
	telemetry.InitMetrics()
	telemetry.InitMetrics()

	telemetry.RecordHTTPRequest("GET", "/api/v1/foods", 200, 0.045, 512)
	telemetry.RecordHTTPRequest("POST", "/api/v1/foods", 201, 0.12, 0) // zero bytes written

	telemetry.DbErrorsTotal.WithLabelValues("disk_locked").Inc()
	telemetry.SessionCleanupsTotal.Inc()
	telemetry.ClientEventsDroppedTotal.WithLabelValues("rate_limit").Inc()

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	expectedSubstrings := []string{
		"calorize_http_requests_total",
		`calorize_http_requests_total{method="GET",route="/api/v1/foods",status="200"}`,
		`calorize_http_requests_total{method="POST",route="/api/v1/foods",status="201"}`,
		"calorize_http_request_duration_seconds",
		`calorize_http_request_duration_seconds_count{method="GET",route="/api/v1/foods",status="200"} 1`,
		"calorize_http_response_size_bytes",
		`calorize_http_response_size_bytes{method="GET",route="/api/v1/foods"} 512`,
		`calorize_db_errors_total{kind="disk_locked"} 1`,
		`calorize_sessions_cleaned_total 1`,
		`calorize_client_events_dropped_total{reason="rate_limit"} 1`,
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(body, sub) {
			t.Errorf("expected metrics body to contain %q, but did not find it in:\n%s", sub, body)
		}
	}
}
