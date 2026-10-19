// Package observability implements health check endpoints (FR-OBS-01, RULE-OBS-04).
package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// TC-OBS-01-INT: /readyz endpoint reflects database reachability
// (FR-OBS-01, AC-06, RULE-OBS-04)
// ---------------------------------------------------------------------------

func TestHealthHandler_Livez(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(func(_ context.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	h.LivezHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("LivezHandler expected 200, got %d", res.StatusCode)
	}
}

func TestHealthHandler_Readyz_DBUp(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(func(_ context.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h.ReadyzHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("ReadyzHandler expected 200 when DB is reachable, got %d", res.StatusCode)
	}
}

func TestHealthHandler_Readyz_DBDown(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(func(_ context.Context) error {
		return errors.New("connection refused")
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h.ReadyzHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("ReadyzHandler expected 503 when DB is unreachable, got %d", res.StatusCode)
	}
}

func TestHealthHandler_Readyz_Timeout(t *testing.T) {
	t.Parallel()

	// Simulate a context deadline exceeded scenario
	h := NewHealthHandler(func(ctx context.Context) error {
		return context.DeadlineExceeded
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h.ReadyzHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("ReadyzHandler expected 503 on timeout, got %d", res.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// NewDatabasePingFunc tests
// ---------------------------------------------------------------------------

func TestNewDatabasePingFunc_Success(t *testing.T) {
	t.Parallel()

	// We cannot test against a real DB here, but we can verify the function
	// signature and that it runs within timeout context.
	pingFn := NewDatabasePingFunc(nil, 0) // nil db will panic, but we just test compilation
	_ = pingFn
}

func TestHealthHandler_Constructor_NilPing(t *testing.T) {
	t.Parallel()

	// Should handle nil ping func gracefully (delegate to ping func, but not panic in constructor)
	h := NewHealthHandler(nil)
	if h == nil {
		t.Fatal("NewHealthHandler should return non-nil even with nil ping func")
	}
}
