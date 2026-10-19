// Package observability implements health check endpoints (FR-OBS-01, RULE-OBS-04).
//
// It exposes:
// - /livez: simple liveness probe returning 200 OK when the process is alive.
// - /readyz: readiness probe that checks database reachability before returning 200 OK.
//
// Both endpoints follow the Kubernetes health probe conventions and are designed
// to be registered via http.ServeMux or any compatible HTTP router.
package observability

import (
	"context"
	"encoding/json"
	"database/sql"
	"net/http"
	"time"
)

// HealthHandler defines HTTP handlers for liveness and readiness probes.
type HealthHandler interface {
	// LivezHandler handles GET /livez (process liveness).
	LivezHandler(w http.ResponseWriter, r *http.Request)

	// ReadyzHandler handles GET /readyz (dependency readiness).
	// Returns 200 OK if all dependencies are healthy, 503 otherwise (AC-06).
	ReadyzHandler(w http.ResponseWriter, r *http.Request)
}

// healthHandler implements HealthHandler with a pluggable DB ping function.
type healthHandler struct {
	dbPingFunc func(ctx context.Context) error
}

// NewHealthHandler creates a new HealthHandler.
// The dbPingFunc is called by ReadyzHandler to verify database reachability.
// If dbPingFunc is nil, readiness always returns 200 (no DB dependency yet).
func NewHealthHandler(dbPingFunc func(ctx context.Context) error) HealthHandler {
	return &healthHandler{dbPingFunc: dbPingFunc}
}

// LivezHandler responds with 200 OK to indicate the process is alive (FR-OBS-01).
func (h *healthHandler) LivezHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"alive"}`))
}

// ReadyzHandler checks dependency health and returns 200 if everything is ready,
// or 503 if a dependency is unavailable (FR-OBS-01, AC-06, RULE-OBS-04).
func (h *healthHandler) ReadyzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// If no ping func is configured, assume ready (no DB dependency yet).
	if h.dbPingFunc == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
		return
	}

	// Check database reachability with a timeout derived from request context.
	if err := h.dbPingFunc(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		resp, _ := json.Marshal(map[string]string{
			"status": "not ready",
			"error":  err.Error(),
		})
		_, _ = w.Write(resp)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

// NewDatabasePingFunc creates a dbPingFunc that pings the database with a timeout.
// The db parameter is the *sql.DB instance. If db is nil, the returned function
// always returns nil (no-op for testing or when DB is not configured).
func NewDatabasePingFunc(db *sql.DB, timeout time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if db == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return db.PingContext(ctx)
	}
}
