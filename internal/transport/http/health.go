package http

import (
	"context"
	"net/http"
	"time"
)

type HealthChecker interface {
	PingContext(ctx context.Context) error
}

type HealthHandler struct {
	db HealthChecker
}

func NewHealthHandler(db HealthChecker) *HealthHandler {
	return &HealthHandler{db: db}
}

// Liveness always returns 200 (process alive).
func (h *HealthHandler) Livez(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Readiness pings DB; returns 503 if unreachable (RULE-OBS-04, AC-06).
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		h.writeReady(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
		return
	}
	h.writeReady(w)
}

func (h *HealthHandler) writeReady(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}
