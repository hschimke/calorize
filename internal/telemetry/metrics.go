package telemetry

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	once sync.Once

	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "calorize_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "route", "status"},
	)

	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "calorize_http_request_duration_seconds",
			Help:    "HTTP request latency histogram in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)

	HttpResponseSize = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "calorize_http_response_size_bytes",
			Help: "Total bytes written in HTTP responses",
		},
		[]string{"method", "route"},
	)

	DbErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "calorize_db_errors_total",
			Help: "Total number of database errors encountered",
		},
		[]string{"kind"},
	)

	SessionCleanupsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "calorize_sessions_cleaned_total",
			Help: "Total expired sessions removed by the background cleanup daemon",
		},
	)

	ClientEventsDroppedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "calorize_client_events_dropped_total",
			Help: "Total client telemetry events dropped",
		},
		[]string{"reason"},
	)
)

// InitMetrics ensures metrics initialization is idempotent.
func InitMetrics() {
	once.Do(func() {
		// Metrics are registered globally via promauto.
	})
}

// RecordHTTPRequest records RED metrics for an incoming HTTP request.
func RecordHTTPRequest(method, route string, status int, durationSec float64, bytesWritten int64) {
	statusStr := strconv.Itoa(status)
	HttpRequestsTotal.WithLabelValues(method, route, statusStr).Inc()
	HttpRequestDuration.WithLabelValues(method, route, statusStr).Observe(durationSec)
	if bytesWritten > 0 {
		HttpResponseSize.WithLabelValues(method, route).Add(float64(bytesWritten))
	}
}
