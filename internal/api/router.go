package api

import (
	"net/http"
)

// NewRouter creates a new HTTP mux with all relaybox routes.
// TenantMiddleware must wrap all business routes.
func NewRouter(eventHandler *EventHandler, subHandler *SubscriptionHandler) http.Handler {
	mux := http.NewServeMux()

	// Event routes.
	mux.HandleFunc("POST /v1/events", eventHandler.HandleCreateEvent)

	// Subscription routes.
	mux.HandleFunc("POST /v1/subscriptions", subHandler.HandleCreate)
	mux.HandleFunc("GET /v1/subscriptions", subHandler.HandleList)
	mux.HandleFunc("DELETE /v1/subscriptions/{id}", subHandler.HandleDelete)

	// Wrap with tenant middleware.
	return TenantMiddleware(mux)
}
