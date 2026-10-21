// Package router configures HTTP routes and applies global middleware
// (Clean Architecture transport layer).
package router

import (
	"net/http"

	"github.com/mariozul/relaybox/pkg/relaybox/handler"
)

// Setup configures all HTTP routes and returns the configured mux.
// Routes:
//   - GET /livez  - liveness probe (FR-OBS-01)
//   - GET /readyz - readiness probe (FR-OBS-01)
//   - POST /v1/events           - event ingestion (FR-ING-01)
//   - POST   /v1/subscriptions  - create subscription (FR-SUB-01)
//   - GET    /v1/subscriptions  - list subscriptions (FR-SUB-01)
//   - GET    /v1/subscriptions/{id} - get subscription (FR-SUB-01)
//   - PUT    /v1/subscriptions/{id} - update subscription (FR-SUB-01)
//   - DELETE /v1/subscriptions/{id} - delete subscription (FR-SUB-01)
//
// Tenant identity is expected to be injected into the request context by
// upstream auth middleware (ASSUMP-01, RULE-SEC-01).
func Setup(
	eventHandler handler.EventHandler,
	subHandler handler.SubscriptionHandler,
	livezHandler, readyzHandler http.HandlerFunc,
) *http.ServeMux {
	mux := http.NewServeMux()

	// Health probes (FR-OBS-01, RULE-OBS-04)
	mux.HandleFunc("GET /livez", livezHandler)
	mux.HandleFunc("GET /readyz", readyzHandler)

	// V1 API routes (tenant-authenticated)
	mux.HandleFunc("POST /v1/events", eventHandler.IngestEventHandler)

	// Subscription CRUD (FR-SUB-01)
	mux.HandleFunc("POST /v1/subscriptions", subHandler.CreateSubscriptionHandler)
	mux.HandleFunc("GET /v1/subscriptions", subHandler.ListSubscriptionsHandler)
	mux.HandleFunc("GET /v1/subscriptions/{id}", subHandler.GetSubscriptionHandler)
	mux.HandleFunc("PUT /v1/subscriptions/{id}", subHandler.UpdateSubscriptionHandler)
	mux.HandleFunc("DELETE /v1/subscriptions/{id}", subHandler.DeleteSubscriptionHandler)

	return mux
}
