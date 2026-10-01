package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RULE-OBS-03: bounded metric cardinality — no tenant IDs or UUIDs as labels.

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_http_requests_total",
			Help: "Total HTTP requests.",
		},
		[]string{"method", "route", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "app_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)

	ingestionEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_ingestion_events_total",
			Help: "Total ingestion events.",
		},
		[]string{"status"},
	)

	deliveryAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_delivery_attempts_total",
			Help: "Total delivery attempts.",
		},
		[]string{"status"},
	)

	deliveryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "app_delivery_duration_seconds",
			Help:    "Delivery duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		ingestionEventsTotal,
		deliveryAttemptsTotal,
		deliveryDuration,
	)
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()
		// RULE-OBS-03: use route pattern (not raw URL) to bound cardinality.
		route := routePattern(r)
		httpRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rw.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(duration)
	})
}

// routePattern returns a bounded route label, avoiding raw URL path as label (RULE-OBS-03).
func routePattern(r *http.Request) string {
	// Use Go 1.22+ routing pattern if available.
	if p := r.Pattern; p != "" {
		return p
	}
	// For Go 1.21-, map known paths to fixed patterns.
	switch {
	case r.URL.Path == "/v1/events":
		return "/v1/events"
	case r.URL.Path == "/v1/subscriptions":
		return "/v1/subscriptions"
	case r.URL.Path == "/livez":
		return "/livez"
	case r.URL.Path == "/readyz":
		return "/readyz"
	case r.URL.Path == "/metrics":
		return "/metrics"
	default:
		return "/unknown"
	}
}
