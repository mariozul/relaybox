package http

import (
	
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler serves /readyz with DB reachability check (FR-OBS-01, AC-06).
type HealthHandler struct {
	pool *pgxpool.Pool
}

func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// ServeHTTP handles GET /readyz and returns 200 if DB is reachable, 503 otherwise.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("no database pool configured"))
		return
	}
	if err := h.pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("database unreachable"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// LiveHandler serves GET /livez.
func LiveHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
