// Package handler implements HTTP transport handlers for the Relaybox
// multi-tenant webhook relay service. Handlers are responsible for request
// parsing, tenant context extraction, and domain error mapping to HTTP
// responses (RULE-ARCH-02, RULE-ARCH-03).
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/metrics"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// ============================================================================
// Tenant Context Management (RULE-SEC-01)
// ============================================================================

// TenantContextKey is the context key type used to store and retrieve the
// authenticated tenant ID from the request context. It is exported so that
// middleware and tests can set and retrieve tenant identity consistently.
type TenantContextKey string

// TenantIDKey is the context key used to store the authenticated tenant ID.
// The value is set by authentication middleware and retrieved by handlers
// (RULE-SEC-01).
const TenantIDKey TenantContextKey = "tenant_id"

// WithTenantContext embeds a tenant ID into the provided context. This should
// be called by authentication middleware after verifying the tenant identity
// from tokens or gateway-verified headers (RULE-SEC-01).
func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// ExtractTenantID extracts the authenticated tenant ID from the request
// context. The tenant identity is injected by authentication middleware and
// must never be parsed from request bodies or query parameters on public
// endpoints (RULE-SEC-01). Returns an empty string if no tenant is found.
func ExtractTenantID(r *http.Request) string {
	if tenantID, ok := r.Context().Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return ""
}

// SubscriptionHandler implements HTTP handlers for CRUD operations on
// subscriptions (FR-SUB-01). It delegates business logic to the
// SubscriptionService and maps domain errors to appropriate HTTP status codes
// (RULE-ARCH-03).
type SubscriptionHandler struct {
	svc service.SubscriptionService
}

// NewSubscriptionHandler creates a new SubscriptionHandler with the given
// SubscriptionService.
func NewSubscriptionHandler(svc service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{
		svc: svc,
	}
}

// HandleCreateSubscription handles POST /v1/subscriptions requests.
// It parses the JSON request body into a CreateSubscriptionRequest, extracts
// the tenant ID from the request context, and delegates to the service layer.
// Returns 201 Created on success with the created subscription in the body.
// Returns 400 Bad Request for invalid payloads.
func (h *SubscriptionHandler) HandleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	tenantID := ExtractTenantID(r)
	if tenantID == "" {
		slog.WarnContext(r.Context(), "missing tenant ID in request context",
			"handler", "subscription",
			"method", r.Method,
			"path", r.URL.Path,
		)
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		metrics.RecordHTTPDuration("POST /v1/subscriptions", r.Method, http.StatusBadRequest, time.Since(start))
		return
	}

	var req service.CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(r.Context(), "failed to decode request body",
			"handler", "subscription",
			"error", err.Error(),
		)
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		metrics.RecordHTTPDuration("POST /v1/subscriptions", r.Method, http.StatusBadRequest, time.Since(start))
		return
	}

	sub, err := h.svc.CreateSubscription(r.Context(), tenantID, req)
	if err != nil {
		writeDomainError(w, err)
		metrics.RecordHTTPDuration("POST /v1/subscriptions", r.Method, mapDomainErrorToHTTPStatus(err), time.Since(start))
		return
	}

	slog.InfoContext(r.Context(), "subscription created",
		"handler", "subscription",
		"subscription_id", sub.ID,
		"tenant_id", tenantID,
		"event_type", req.EventType,
	)
	metrics.RecordHTTPDuration("POST /v1/subscriptions", r.Method, http.StatusCreated, time.Since(start))
	writeJSON(w, http.StatusCreated, sub)
}

// HandleGetSubscription handles GET /v1/subscriptions/{id} requests.
// It extracts the subscription ID from the URL path, retrieves the tenant
// from context, and fetches the subscription. Returns 200 OK with the
// subscription on success, 404 Not Found if the subscription does not exist
// or is owned by a different tenant (RULE-SEC-01).
func (h *SubscriptionHandler) HandleGetSubscription(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	tenantID := ExtractTenantID(r)
	if tenantID == "" {
		slog.WarnContext(r.Context(), "missing tenant ID in request context",
			"handler", "subscription",
			"method", r.Method,
			"path", r.URL.Path,
		)
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		metrics.RecordHTTPDuration("GET /v1/subscriptions/{id}", r.Method, http.StatusBadRequest, time.Since(start))
		return
	}

	// Extract subscription ID from URL path: /v1/subscriptions/{id}
	id := extractPathParam(r.URL.Path, "/v1/subscriptions/")
	if id == "" {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		metrics.RecordHTTPDuration("GET /v1/subscriptions/{id}", r.Method, http.StatusBadRequest, time.Since(start))
		return
	}

	sub, err := h.svc.GetSubscription(r.Context(), tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		metrics.RecordHTTPDuration("GET /v1/subscriptions/{id}", r.Method, mapDomainErrorToHTTPStatus(err), time.Since(start))
		return
	}

	slog.InfoContext(r.Context(), "subscription retrieved",
		"handler", "subscription",
		"subscription_id", id,
		"tenant_id", tenantID,
	)
	metrics.RecordHTTPDuration("GET /v1/subscriptions/{id}", r.Method, http.StatusOK, time.Since(start))
	writeJSON(w, http.StatusOK, sub)
}

// HandleListSubscriptions handles GET /v1/subscriptions requests.
// It retrieves all subscriptions owned by the authenticated tenant.
// Returns 200 OK with the subscription list (possibly empty).
func (h *SubscriptionHandler) HandleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	tenantID := ExtractTenantID(r)
	if tenantID == "" {
		slog.WarnContext(r.Context(), "missing tenant ID in request context",
			"handler", "subscription",
			"method", r.Method,
			"path", r.URL.Path,
		)
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		metrics.RecordHTTPDuration("GET /v1/subscriptions", r.Method, http.StatusBadRequest, time.Since(start))
		return
	}

	subs, err := h.svc.ListSubscriptions(r.Context(), tenantID)
	if err != nil {
		writeDomainError(w, err)
		metrics.RecordHTTPDuration("GET /v1/subscriptions", r.Method, mapDomainErrorToHTTPStatus(err), time.Since(start))
		return
	}

	// Return empty JSON array instead of null for consistent API responses.
	if subs == nil {
		subs = make([]domain.Subscription, 0)
	}

	slog.InfoContext(r.Context(), "subscriptions listed",
		"handler", "subscription",
		"tenant_id", tenantID,
		"count", len(subs),
	)
	metrics.RecordHTTPDuration("GET /v1/subscriptions", r.Method, http.StatusOK, time.Since(start))
	writeJSON(w, http.StatusOK, subs)
}

// HandleDeleteSubscription handles DELETE /v1/subscriptions/{id} requests.
// It extracts the subscription ID from the URL path, retrieves the tenant
// from context, and deletes the subscription. Returns 204 No Content on
// success, 404 Not Found if the subscription does not exist or is owned by
// a different tenant (RULE-SEC-01).
func (h *SubscriptionHandler) HandleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	tenantID := ExtractTenantID(r)
	if tenantID == "" {
		slog.WarnContext(r.Context(), "missing tenant ID in request context",
			"handler", "subscription",
			"method", r.Method,
			"path", r.URL.Path,
		)
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		metrics.RecordHTTPDuration("DELETE /v1/subscriptions/{id}", r.Method, http.StatusBadRequest, time.Since(start))
		return
	}

	// Extract subscription ID from URL path: /v1/subscriptions/{id}
	id := extractPathParam(r.URL.Path, "/v1/subscriptions/")
	if id == "" {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		metrics.RecordHTTPDuration("DELETE /v1/subscriptions/{id}", r.Method, http.StatusBadRequest, time.Since(start))
		return
	}

	err := h.svc.DeleteSubscription(r.Context(), tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		metrics.RecordHTTPDuration("DELETE /v1/subscriptions/{id}", r.Method, mapDomainErrorToHTTPStatus(err), time.Since(start))
		return
	}

	slog.InfoContext(r.Context(), "subscription deleted",
		"handler", "subscription",
		"subscription_id", id,
		"tenant_id", tenantID,
	)
	metrics.RecordHTTPDuration("DELETE /v1/subscriptions/{id}", r.Method, http.StatusNoContent, time.Since(start))
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// Helpers
// ============================================================================

// extractPathParam extracts a path parameter from the URL path by removing
// the given prefix. For example, extractPathParam("/v1/subscriptions/sub-123",
// "/v1/subscriptions/") returns "sub-123". Returns an empty string if the
// path does not have the expected prefix or no parameter follows.
func extractPathParam(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	param := strings.TrimPrefix(path, prefix)
	// Remove any trailing slash or additional path segments.
	if idx := strings.Index(param, "/"); idx != -1 {
		param = param[:idx]
	}
	return param
}
