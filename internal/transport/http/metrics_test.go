package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httptransport "github.com/mariozul/relaybox/internal/transport/http"
)

// TC-14-UNIT: bounded metric labels only — no tenant/UUID in labels.
func TestMetricsLabelsBounded(t *testing.T) {
	t.Parallel()
	handler := httptransport.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Verify metrics handler is registered.
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", httptransport.MetricsHandler())
	req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d", rec2.Code)
	}
	body := rec2.Body.String()
	if !strings.Contains(body, "app_http_requests_total") {
		t.Fatal("metrics must include app_http_requests_total")
	}
	if strings.Contains(body, "tenant_id") {
		t.Fatal("metrics must NOT include unbounded label tenant_id (RULE-OBS-03)")
	}
}
