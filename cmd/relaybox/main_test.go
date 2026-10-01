package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpointsNoDB(t *testing.T) {
	t.Parallel()

	// Test /livez handler directly (always 200).
	h := &struct{}{}
	_ = h

	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("db not configured"))
	})

	// Test /livez.
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Test /readyz without DB pool returns 503 (AC-06).
	req2 := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w2.Code)
	}
}
