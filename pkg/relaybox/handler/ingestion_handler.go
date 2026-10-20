// Package handler implements HTTP transport handlers for the Relaybox
// multi-tenant webhook relay service (Clean Architecture transport layer).
//
// Handlers are responsible for:
//   - Parsing HTTP requests and validating input structure
//   - Extracting tenant identity from the authenticated request context
//     (RULE-SEC-01)
//   - Invoking service layer methods
//   - Mapping domain errors to appropriate HTTP status codes (RULE-ARCH-03)
//   - Writing structured JSON responses
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// CtxKeyTenantID is the context key used to store and retrieve the
// authenticated tenant ID from the request context. The tenant identity
// MUST be resolved from the authenticated request context (e.g., by an
// authentication middleware), never from the request body (RULE-SEC-01).
type CtxKeyTenantIDType string

const CtxKeyTenantID CtxKeyTenantIDType = "tenant_id"

// IngestionService defines the interface for the event ingestion use case
// that the handler depends on. This follows Clean Architecture dependency
// inversion (RULE-ARCH-02) - the handler depends on an interface, not a
// concrete service implementation.
type IngestionService interface {
	IngestEvent(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error)
}

// IngestionHandler handles HTTP requests for event ingestion via
// POST /v1/events (FR-ING-01).
type IngestionHandler struct {
	svc IngestionService
}

// NewIngestionHandler creates a new IngestionHandler with the given
// IngestionService dependency. It panics if svc is nil to fail fast
// on misconfiguration.
func NewIngestionHandler(svc IngestionService) *IngestionHandler {
	if svc == nil {
		panic("ingestion handler: service must not be nil")
	}
	return &IngestionHandler{svc: svc}
}

// ingestEventRequest represents the JSON request body expected by
// POST /v1/events. It mirrors service.IngestEventRequest but uses
// json.RawMessage for the payload field to preserve the raw JSON
// structure for validation and forwarding.
type ingestEventRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	DedupKey  string          `json:"dedup_key"`
}

// HandleIngestEvent handles the POST /v1/events endpoint.
//
// It performs the following steps:
//  1. Extracts the tenant ID from the authenticated request context
//     (RULE-SEC-01). Returns 403 Forbidden if not present.
//  2. Decodes the JSON request body into an ingestEventRequest struct.
//     Returns 400 Bad Request on malformed JSON.
//  3. Validates required fields (event_type, payload, dedup_key).
//     Returns 400 Bad Request on validation failure.
//  4. Converts the request to a service.IngestEventRequest and invokes
//     the IngestionService.
//  5. Maps domain errors to HTTP status codes (RULE-ARCH-03):
//     - ErrInvalidEventPayload -> 400 Bad Request
//     - ErrDuplicateEvent -> 409 Conflict
//     - ErrUnauthorized -> 403 Forbidden
//     - ErrInternal -> 500 Internal Server Error
//  6. Writes the response as JSON with Content-Type: application/json.
func (h *IngestionHandler) HandleIngestEvent(w http.ResponseWriter, r *http.Request) {
	// Step 1: Extract tenant identity from context (RULE-SEC-01)
	tenantID, err := extractTenantID(r.Context())
	if err != nil {
		writeError(w, http.StatusForbidden, domain.ErrUnauthorized)
		return
	}

	// Step 2: Decode request body
	var req ingestEventRequest
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.DebugContext(r.Context(), "failed to decode request body",
			"error", err,
			"component", "ingestion_handler",
		)
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		return
	}

	// Step 3: Validate required fields
	// event_type must be non-empty
	if req.EventType == "" {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		return
	}

	// payload must be present and non-null (valid JSON value)
	// json.RawMessage("null") has len 4, json.RawMessage(nil) means absent
	if len(req.Payload) == 0 || bytes.Equal(req.Payload, []byte("null")) {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		return
	}

	// dedup_key must be non-empty
	if req.DedupKey == "" {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		return
	}

	// Validate that payload is valid JSON
	if !json.Valid(req.Payload) {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidEventPayload)
		return
	}

	// Step 4: Invoke ingestion service
	svcReq := service.IngestEventRequest{
		EventType: req.EventType,
		Payload:   req.Payload,
		DedupKey:  req.DedupKey,
	}

	resp, err := h.svc.IngestEvent(r.Context(), tenantID, svcReq, time.Now().UTC())
	if err != nil {
		// Step 5: Map domain errors to HTTP status codes (RULE-ARCH-03)
		statusCode := mapDomainErrorToHTTPStatus(err)
		writeError(w, statusCode, err)
		return
	}

	// Step 6: Write success response
	writeJSON(w, http.StatusOK, map[string]string{
		"event_id": resp.EventID,
	})
}

// extractTenantID extracts the tenant ID from the request context.
// It returns the tenant ID as a string, or an error if the tenant ID
// is missing or invalid (RULE-SEC-01).
func extractTenantID(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", domain.ErrUnauthorized
	}

	tenantID, ok := ctx.Value(CtxKeyTenantID).(string)
	if !ok || tenantID == "" {
		return "", domain.ErrUnauthorized
	}

	return tenantID, nil
}

// mapDomainErrorToHTTPStatus maps domain errors to HTTP status codes
// following the error catalog specified in the requirements (RULE-ARCH-03).
func mapDomainErrorToHTTPStatus(err error) int {
	switch {
	case errors.Is(err, domain.ErrInvalidEventPayload):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrDuplicateEvent):
		return http.StatusConflict
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// writeError writes a structured JSON error response with the given HTTP
// status code and domain error. The response body contains "code" and
// "message" fields extracted from the domain error (RULE-ARCH-03).
func writeError(w http.ResponseWriter, statusCode int, err error) {
	// Extract code and message from domain error
	code := "RELAYBOX_INTERNAL_ERROR"
	message := "an unexpected internal server error occurred"

	var domainErr *domain.DomainError
	if errors.As(err, &domainErr) {
		code = domainErr.Code
		message = domainErr.Message
	}

	writeJSON(w, statusCode, map[string]string{
		"code":    code,
		"message": message,
	})
}

// writeJSON writes a JSON response with the given HTTP status code and
// payload. It sets the Content-Type header to application/json.
func writeJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			slog.Error("failed to encode JSON response",
				"error", err,
				"component", "ingestion_handler",
				"status_code", statusCode,
			)
		}
	}
}
