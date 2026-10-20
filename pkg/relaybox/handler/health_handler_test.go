package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mariozul/relaybox/pkg/relaybox/handler"
)

// ============================================================================
// TC-08-EDGE: /livez and /readyz health endpoints (FR-OBS-01, RULE-OBS-04)
// ============================================================================

// mockDBChecker implements handler.DBChecker for testing.
type mockDBChecker struct {
	checkDBFn func(ctx context.Context) error
}

func (m *mockDBChecker) CheckDB(ctx context.Context) error {
	if m.checkDBFn != nil {
		return m.checkDBFn(ctx)
	}
	return nil
}

func TestHealthHandler_HandleLivez_Returns200(t *testing.T) {
	t.Parallel()

	dbChecker := &mockDBChecker{}
	h := handler.NewHealthHandler(dbChecker)

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rr := httptest.NewRecorder()

	h.HandleLivez(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rr.Body.String())
	}
}

func TestHealthHandler_HandleReadyz_Returns200WhenDBIsReachable(t *testing.T) {
	t.Parallel()

	dbChecker := &mockDBChecker{
		checkDBFn: func(ctx context.Context) error {
			return nil
		},
	}
	h := handler.NewHealthHandler(dbChecker)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.HandleReadyz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "ready" {
		t.Errorf("expected body 'ready', got %q", rr.Body.String())
	}
}

func TestHealthHandler_HandleReadyz_Returns503WhenDBIsUnreachable(t *testing.T) {
	t.Parallel()

	dbChecker := &mockDBChecker{
		checkDBFn: func(ctx context.Context) error {
			return errors.New("database connection refused")
		},
	}
	h := handler.NewHealthHandler(dbChecker)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.HandleReadyz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "unavailable" {
		t.Errorf("expected body 'unavailable', got %q", rr.Body.String())
	}
}

func TestHealthHandler_HandleReadyz_Returns503WhenDBCheckerIsNil(t *testing.T) {
	t.Parallel()

	h := handler.NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.HandleReadyz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503 when DBChecker is nil, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "unavailable" {
		t.Errorf("expected body 'unavailable', got %q", rr.Body.String())
	}
}
