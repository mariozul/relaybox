package http_test

import (
	"errors"
	"net/http"
	"context"
	"net/http/httptest"
	"testing"

	httptransport "github.com/mariozul/relaybox/internal/transport/http"
)

type stubHealthDB struct {
	err error
}

func (s *stubHealthDB) PingContext(ctx context.Context) error {
	return s.err
}

// TC-13-INT: /readyz returns 200 with healthy DB, 503 on DB down (AC-06).
func TestHealthReadyz200(t *testing.T) {
	t.Parallel()
	h := httptransport.NewHealthHandler(&stubHealthDB{err: nil})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.Readyz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHealthReadyz503(t *testing.T) {
	t.Parallel()
	h := httptransport.NewHealthHandler(&stubHealthDB{err: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.Readyz(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on DB down, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHealthLivez200(t *testing.T) {
	t.Parallel()
	h := httptransport.NewHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	h.Livez(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
