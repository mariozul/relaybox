package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivez(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	h.Livez(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadyz_NoPool(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h.Readyz(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without pool, got %d", w.Code)
	}
}
