package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds bounded-cardinality Prometheus metrics (RULE-OBS-03).
type Metrics struct {
	RequestDuration *prometheus.HistogramVec
	RequestTotal    *prometheus.CounterVec
	DeliveryTotal   *prometheus.CounterVec
}

// NewMetrics creates and registers Prometheus metrics with bounded-cardinality labels.
func NewMetrics() *Metrics {
	m := &Metrics{
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "relaybox_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "status"}),
		RequestTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "relaybox_http_requests_total",
			Help: "Total HTTP requests served.",
		}, []string{"method", "status"}),
		DeliveryTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "relaybox_delivery_attempts_total",
			Help: "Total delivery attempts by outcome.",
		}, []string{"status"}),
	}

	prometheus.MustRegister(m.RequestDuration)
	prometheus.MustRegister(m.RequestTotal)
	prometheus.MustRegister(m.DeliveryTotal)

	return m
}

// MetricsHandler returns an HTTP handler for Prometheus scraping.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordRequest records an HTTP request metric.
func (m *Metrics) RecordRequest(method string, statusCode int) {
	status := "success"
	if statusCode >= 400 {
		status = "error"
	}
	m.RequestTotal.WithLabelValues(method, status).Inc()
}

// RecordDelivery records a delivery attempt outcome.
func (m *Metrics) RecordDelivery(status string) {
	m.DeliveryTotal.WithLabelValues(status).Inc()
}
