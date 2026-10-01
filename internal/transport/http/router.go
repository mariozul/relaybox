package http

import (
	"net/http"
)

// RegisterRoutes wires all HTTP routes onto the given mux.
func RegisterRoutes(mux *http.ServeMux,
	ingest *IngestHandler,
	subscription *SubscriptionHandler,
	health *HealthHandler,
) {
	// Liveness/readiness probes (no auth).
	mux.HandleFunc("/livez", LiveHandler)
	mux.Handle("/readyz", health)

	// Tenant-scoped API endpoints (RULE-SEC-01).
	tenantMux := http.NewServeMux()
	tenantMux.Handle("/v1/events", ingest)
	tenantMux.Handle("/v1/subscriptions", subscription)
	mux.Handle("/v1/", TenantMiddleware(tenantMux))
}
