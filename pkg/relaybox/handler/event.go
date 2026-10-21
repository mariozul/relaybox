// Package handler implements HTTP transport handlers (Clean Architecture transport layer).
// Handlers parse HTTP requests, extract tenant identity from context (RULE-SEC-01),
// delegate to application services, and map domain errors to HTTP status codes.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/dto"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// EventHandler defines HTTP handlers for event ingestion.
type EventHandler interface {
	// IngestEventHandler handles POST /v1/events (FR-ING-01).
	// Tenant identity MUST be resolved from request context, never from body (RULE-SEC-01).
	IngestEventHandler(w http.ResponseWriter, r *http.Request)
}

type eventHandler struct {
	ingestionService service.EventIngestionService
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(ingestionService service.EventIngestionService) EventHandler {
	return &eventHandler{ingestionService: ingestionService}
}

// IngestEventHandler handles POST /v1/events.
//
// Request flow:
//  1. Extract tenant ID from request context (RULE-SEC-01: ASSUMP-01 — auth middleware
//     populates context with tenant_id; never from request body).
//  2. Decode JSON request body into IngestEventRequest.
//  3. Call EventIngestionService.IngestEvent.
//  4. Map domain errors to appropriate HTTP status codes (Error Catalog, RULE-ARCH-03).
//  5. Return JSON response with event_id.
func (h *eventHandler) IngestEventHandler(w http.ResponseWriter, r *http.Request) {
	// Step 1: Extract tenant identity from context (RULE-SEC-01)
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		slog.WarnContext(r.Context(), "missing tenant_id in context",
			"path", r.URL.Path,
			"method", r.Method,
		)
		writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized: missing tenant identity")
		return
	}

	// Step 2: Decode request body
	var req dto.IngestEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(r.Context(), "invalid JSON in event ingestion request",
			"path", r.URL.Path,
			"error", err.Error(),
		)
		writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, "invalid JSON request body")
		return
	}

	// Step 3: Delegate to service
	eventID, err := h.ingestionService.IngestEvent(r.Context(), tenantID, &req)
	if err != nil {
		// Step 4: Map domain errors to HTTP status codes
		switch {
		case errors.Is(err, domain.ErrEventAlreadyExists):
			// Idempotent resend: return 409 Conflict with the original event ID (FR-ING-03)
			writeJSON(w, http.StatusConflict, map[string]string{
				"event_id": eventID,
				"message":  "event already exists; this is the original event ID",
			})
			return
		case errors.Is(err, domain.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, err.Error())
			return
		case errors.Is(err, domain.ErrUnauthorized):
			writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized access")
			return
		default:
			// Internal/unexpected error
			slog.ErrorContext(r.Context(), "event ingestion failed",
				"tenant_id", tenantID,
				"error", err.Error(),
			)
			writeError(w, http.StatusInternalServerError, domain.ErrCodeInternal, "internal server error")
			return
		}
	}

	// Step 5: Success response
	writeJSON(w, http.StatusOK, map[string]string{
		"event_id": eventID,
	})
}
