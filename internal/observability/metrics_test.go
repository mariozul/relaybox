package observability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetrics_BoundedCardinality(t *testing.T) {
	t.Parallel()

	m := NewMetrics()

	// Verify only bounded labels are used (RULE-OBS-03).
	m.RecordRequest("POST", 201)
	m.RecordRequest("POST", 500)
	m.RecordDelivery("delivered")
	m.RecordDelivery("failed")
	m.RecordDelivery("deadletter")

	// Scrape metrics endpoint.
	handler := MetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)

	// Must contain our metric names.
	if !strings.Contains(text, "relaybox_http_requests_total") {
		t.Fatal("missing relaybox_http_requests_total metric")
	}
	if !strings.Contains(text, "relaybox_delivery_attempts_total") {
		t.Fatal("missing relaybox_delivery_attempts_total metric")
	}
	// Must NOT contain unbounded labels like tenant IDs.
	if strings.Contains(text, "tenant-a") {
		t.Fatal("metric labels must not contain tenant IDs (RULE-OBS-03 cardiality violation)")
	}
}

func TestNewLogger(t *testing.T) {
	t.Parallel()
	logger := NewLogger(0) // INFO level
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}
