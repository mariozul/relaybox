// Package handler implements HTTP transport handlers for the Relaybox
// multi-tenant webhook relay service.
package handler

import (
	"context"
	"log/slog"
	"net/http"
)

// ============================================================================
// DBChecker interface for readiness probe (FR-OBS-01, RULE-OBS-04)
// ============================================================================

// DBChecker defines the interface for checking database reachability. This
// is used by the HealthHandler to determine if the service is ready to
// accept traffic (RULE-OBS-04).
type DBChecker interface {
	// CheckDB verifies that the database is reachable and responsive. It
	// returns nil if the database is healthy, or an error if the database
	// is unreachable or unhealthy.
	CheckDB(ctx context.Context) error
}

// ============================================================================
// HealthHandler (FR-OBS-01, RULE-OBS-04)
// ============================================================================

// HealthHandler implements HTTP handlers for liveness and readiness probes.
// It follows Clean Architecture (RULE-ARCH-02) by depending on the DBChecker
// interface rather than a concrete database implementation.
//
// Endpoints:
//   - GET /livez: Returns 200 OK when the process is alive (RULE-OBS-04).
//   - GET /readyz: Returns 200 OK when the service is ready to accept
//     traffic (database is reachable). Returns 503 Service Unavailable
//     if the database is unreachable (TC-08-EDGE, AC-06, RULE-OBS-04).
type HealthHandler struct {
	dbChecker DBChecker
}

// NewHealthHandler creates a new HealthHandler with the given DBChecker.
// If dbChecker is nil, the /readyz endpoint will always return 503
// Service Unavailable to prevent the service from accepting traffic
// without a configured database health check.
func NewHealthHandler(dbChecker DBChecker) *HealthHandler {
	return &HealthHandler{
		dbChecker: dbChecker,
	}
}

// HandleLivez handles GET /livez requests. It always returns 200 OK as long
// as the process is running (RULE-OBS-04).
func (h *HealthHandler) HandleLivez(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// HandleReadyz handles GET /readyz requests. It checks database reachability
// via the DBChecker interface (TC-08-EDGE, AC-06, RULE-OBS-04):
//   - If the DBChecker is nil (not configured): returns 503.
//   - If the database is reachable: returns 200 OK with body "ready".
//   - If the database is unreachable: returns 503 Service Unavailable with
//     body "unavailable".
func (h *HealthHandler) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	if h.dbChecker == nil {
		slog.WarnContext(r.Context(), "readiness check skipped: dbChecker not configured",
			"component", "health_handler",
			"endpoint", "/readyz",
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("unavailable"))
		return
	}

	if err := h.dbChecker.CheckDB(r.Context()); err != nil {
		slog.WarnContext(r.Context(), "readiness check failed",
			"component", "health_handler",
			"endpoint", "/readyz",
			"error", err.Error(),
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("unavailable"))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}
