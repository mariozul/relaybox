package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mariozul/relaybox/internal/service"
)

// NewRouter creates the HTTP router with all endpoints.
func NewRouter(eventSvc *service.EventService, subSvc *service.SubscriptionService, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(TenantMiddleware)

	ingest := NewIngestHandler(eventSvc, logger)
	subHandler := NewSubscriptionHandler(subSvc, logger)

	// Liveness & readiness (RULE-OBS-04).
	r.Get("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		// TODO: actual DB ping for true readiness check (FR-OBS-01, AC-06).
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// API v1
	r.Route("/v1", func(r chi.Router) {
		r.Post("/events", ingest.ServeHTTP)
		r.Post("/subscriptions", subHandler.Create)
		r.Get("/subscriptions", subHandler.List)
		r.Delete("/subscriptions/{id}", subHandler.Delete)
	})

	return r
}
