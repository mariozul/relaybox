// Package metrics provides Prometheus metric definitions and recording
// functions for the Relaybox service. All metrics use strictly bounded
// cardinality labels to comply with RULE-OBS-03.
//
// Usage:
//
//	// Production: register with default registerer.
//	metrics.RegisterMetrics(prometheus.DefaultRegisterer)
//
//	// Test: register with isolated registry.
//	reg := prometheus.NewRegistry()
//	metrics.RegisterMetrics(reg)
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metric names are defined as constants to ensure consistency across the
// codebase and avoid drift between metric registration and usage.
const (
	// MetricHTTPRequestDuration is the Prometheus name for the HTTP request
	// duration histogram. Labels: path, method, status_code (bounded).
	MetricHTTPRequestDuration = "relaybox_http_request_duration_seconds"

	// MetricDeliveryOutcomeTotal is the Prometheus name for the delivery
	// outcome counter. Labels: outcome (bounded: delivered|failed|deadletter).
	MetricDeliveryOutcomeTotal = "relaybox_delivery_outcome_total"
)

// Package-level metrics. They are lazily initialized on first call to
// RegisterMetrics or when first accessed via the recording functions.
// In production, the metrics are registered with prometheus.DefaultRegisterer
// via RegisterMetrics. In tests, isolated registries can be used.
var (
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    MetricHTTPRequestDuration,
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method", "status_code"},
	)

	deliveryOutcomeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricDeliveryOutcomeTotal,
			Help: "Total delivery outcomes by status",
		},
		[]string{"outcome"},
	)
)

// RegisterMetrics explicitly registers all Relaybox metrics with the given
// prometheus.Registerer. In production, call with prometheus.DefaultRegisterer.
// In tests, call with prometheus.NewRegistry() to use an isolated registry.
func RegisterMetrics(reg prometheus.Registerer) {
	reg.MustRegister(httpRequestDuration)
	reg.MustRegister(deliveryOutcomeTotal)
}

// RecordHTTPDuration records the duration of an HTTP request, tagged with
// bounded-cardinality labels (path, method, status_code). The caller MUST
// ensure that path is a route pattern (not a raw path with variable segments)
// to avoid unbounded label values (RULE-OBS-03).
func RecordHTTPDuration(path, method string, statusCode int, duration time.Duration) {
	httpRequestDuration.WithLabelValues(
		path,
		method,
		http.StatusText(statusCode),
	).Observe(duration.Seconds())
}

// RecordDeliveryOutcome increments the delivery outcome counter for the given
// outcome. The outcome parameter MUST be one of: "delivered", "failed", or
// "deadletter" to maintain bounded cardinality (RULE-OBS-03).
func RecordDeliveryOutcome(outcome string) {
	deliveryOutcomeTotal.WithLabelValues(outcome).Inc()
}
