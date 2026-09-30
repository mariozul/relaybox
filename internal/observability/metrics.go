package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Requests   *prometheus.HistogramVec
	Deliveries *prometheus.CounterVec
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		Requests:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "relaybox_http_request_duration_seconds", Help: "HTTP request duration."}, []string{"method", "route", "status"}),
		Deliveries: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "relaybox_delivery_total", Help: "Webhook delivery outcomes."}, []string{"status"}),
	}
	reg.MustRegister(m.Requests, m.Deliveries)
	return m
}

func ValidDeliveryStatus(status string) bool {
	return status == "delivered" || status == "failed" || status == "deadletter"
}

func (m *Metrics) Handler(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		m.Requests.WithLabelValues(r.Method, route, strconv.Itoa(recorder.status)).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
