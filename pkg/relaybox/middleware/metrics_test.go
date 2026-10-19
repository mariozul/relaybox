// Package middleware implements HTTP middleware for logging and metrics.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// metricsMu serialises metrics tests that share global Prometheus metric vars.
var metricsMu sync.Mutex

// ---------------------------------------------------------------------------
// TC-OBS-02-UNIT: Bounded-cardinality Prometheus metrics (RULE-OBS-03)
// ---------------------------------------------------------------------------

func TestMetricsMiddleware_RecordsTotalAndDuration(t *testing.T) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	handler := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	expected := strings.NewReader(`
		# HELP http_requests_total Total number of HTTP requests.
		# TYPE http_requests_total counter
		http_requests_total{method="GET",path="/livez",status="200"} 1
	`)
	if err := testutil.CollectAndCompare(httpRequestsTotal, expected); err != nil {
		t.Errorf("unexpected counter value: %v", err)
	}
}

func TestMetricsMiddleware_ErrorStatusRecorded(t *testing.T) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	handler := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/events", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	expected := strings.NewReader(`
		# HELP http_requests_total Total number of HTTP requests.
		# TYPE http_requests_total counter
		http_requests_total{method="POST",path="/v1/events",status="500"} 1
	`)
	if err := testutil.CollectAndCompare(httpRequestsTotal, expected); err != nil {
		t.Errorf("unexpected counter for error status: %v", err)
	}
}

func TestMetricsMiddleware_ConcurrentSafety(t *testing.T) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	handler := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Run requests concurrently to verify no race conditions (RULE-RES-02)
	done := make(chan struct{})
	for range 10 {
		go func() {
			defer func() { done <- struct{}{} }()
			req := httptest.NewRequest(http.MethodGet, "/livez", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			w.Result().Body.Close()
		}()
	}
	for range 10 {
		<-done
	}

	expected := strings.NewReader(`
		# HELP http_requests_total Total number of HTTP requests.
		# TYPE http_requests_total counter
		http_requests_total{method="GET",path="/livez",status="200"} 10
	`)
	if err := testutil.CollectAndCompare(httpRequestsTotal, expected); err != nil {
		t.Errorf("unexpected counter after concurrent requests: %v", err)
	}
}

func TestMetricsMiddleware_HistogramRecorded(t *testing.T) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	httpRequestDuration.Reset()
	httpRequestsTotal.Reset()

	handler := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	// Verify histogram has observations by gathering metrics
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "http_request_duration_seconds" {
			found = true
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h != nil && h.GetSampleCount() > 0 {
					return // found a histogram with observations
				}
			}
		}
	}
	if !found {
		t.Error("http_request_duration_seconds histogram not found in gathered metrics")
	}
}

func TestMetricsMiddleware_DurationsObserved(t *testing.T) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	httpRequestDuration.Reset()
	httpRequestsTotal.Reset()

	handler := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "http_request_duration_seconds" {
			found = true
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h != nil && h.GetSampleCount() > 0 {
					return // found a histogram with observations
				}
			}
		}
	}
	if !found {
		t.Error("http_request_duration_seconds histogram not found in gathered metrics")
	}
}
