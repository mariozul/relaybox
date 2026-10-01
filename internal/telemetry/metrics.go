package telemetry

import "github.com/prometheus/client_golang/prometheus"
import "github.com/prometheus/client_golang/prometheus/promauto"

var (
	// RequestsTotal counts HTTP requests by method, path, status (bounded labels per RULE-OBS-03).
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "relaybox_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	// RequestsDuration tracks request latency histogram.
	RequestsDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relaybox_requests_duration_seconds",
		Help:    "Request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// DeliveryTotal counts delivery outcomes with bounded labels (FR-OBS-02, RULE-OBS-03).
	DeliveryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "relaybox_delivery_total",
		Help: "Total delivery attempts by outcome.",
	}, []string{"status"}) // bounded: delivered, failed, dead_letter
)
