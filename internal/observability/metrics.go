package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics provides bounded-cardinality Prometheus counters/histograms.
// Names: app_{app}_{domain}_{operation}_total | _duration_seconds.
// Labels strictly bounded to {"status":"success"|"failure"} (RULE-OBS-03).
type Metrics struct {
	registry   *prometheus.Registry
	app        string
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
}

var defaultBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
}

// NewMetrics creates a Metrics scoped to app.
func NewMetrics(app string) *Metrics {
	return &Metrics{
		registry:   prometheus.NewRegistry(),
		app:        app,
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
	}
}

func (m *Metrics) key(domain, operation string) string {
	return domain + "/" + operation
}

// Counter returns a CounterVec (label: status) (RULE-OBS-03).
func (m *Metrics) Counter(domain, operation string) *prometheus.CounterVec {
	k := m.key(domain, operation)
	if c, ok := m.counters[k]; ok {
		return c
	}
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_" + m.app + "_" + domain + "_" + operation + "_total",
		Help: "Total count of " + domain + " " + operation + " operations.",
	}, []string{"status"})
	m.registry.MustRegister(cv)
	m.counters[k] = cv
	return cv
}

// Histogram returns a HistogramVec (label: status) (RULE-OBS-03).
func (m *Metrics) Histogram(domain, operation string) *prometheus.HistogramVec {
	k := m.key(domain, operation)
	if h, ok := m.histograms[k]; ok {
		return h
	}
	hv := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "app_" + m.app + "_" + domain + "_" + operation + "_duration_seconds",
		Help:    "Duration histogram for " + domain + " " + operation + " in seconds.",
		Buckets: defaultBuckets,
	}, []string{"status"})
	m.registry.MustRegister(hv)
	m.histograms[k] = hv
	return hv
}

// Registry returns the underlying prometheus.Registry.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// HTTPHandler returns an http.Handler exposing Prometheus metrics.
func (m *Metrics) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
