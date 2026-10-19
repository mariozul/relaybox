// Package middleware implements HTTP middleware for structured logging
// (RULE-OBS-01) and Prometheus metrics (RULE-OBS-03) for the Relaybox service.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics definitions with strictly bounded cardinality labels (RULE-OBS-03).
//
// Allowed label values for http_requests_total and http_request_duration_seconds:
//   - method: HTTP method (GET, POST, PUT, DELETE, etc.)
//   - path:   URL path pattern (e.g., /v1/events, /v1/subscriptions/{id})
//   - status: HTTP status code as string (200, 404, 500, etc.)
//
// These labels are bounded and NEVER include dynamic values such as user IDs,
// tenant IDs, UUIDs, or query parameters (RULE-OBS-03).
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
)

// MetricsMiddleware returns an HTTP middleware that records Prometheus metrics
// for every HTTP request (RULE-OBS-03).
//
// Metrics recorded:
//   - http_requests_total: Counter incremented per request, labeled by method, path, status.
//   - http_request_duration_seconds: Histogram of request durations in seconds.
//
// The middleware wraps the http.ResponseWriter to capture the response status code.
func MetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			status := strconv.Itoa(rw.statusCode)

			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration.Seconds())
		})
	}
}

// metricsResponseWriter wraps http.ResponseWriter to capture the status code.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code before delegating to the wrapped writer.
func (rw *metricsResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}
