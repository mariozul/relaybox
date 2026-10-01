package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/mariozul/relaybox/internal/service"
)

// eventIngester is the interface the handler needs from EventService.
type eventIngester interface {
	Ingest(ctx context.Context, input service.IngestInput) (*service.IngestResult, error)
}

// EventHandler handles HTTP event ingestion.
type EventHandler struct {
	svc eventIngester
}

// NewEventHandler creates an EventHandler.
func NewEventHandler(svc eventIngester) *EventHandler {
	return &EventHandler{svc: svc}
}

// HandleCreateEvent handles POST /v1/events.
func (h *EventHandler) HandleCreateEvent(w http.ResponseWriter, r *http.Request) {
	tenantID := GetTenantID(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "tenant identity required")
		return
	}

	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "cannot parse request body")
		return
	}

	result, err := h.svc.Ingest(r.Context(), service.IngestInput{
		TenantID:  tenantID,
		EventType: req.EventType,
		Payload:   req.Payload,
		DedupKey:  req.DedupKey,
		CreatedBy: tenantID,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, CreateEventResponse{
		EventID:        result.EventID,
		OutboxRowCount: result.OutboxRowCount,
	})
}
