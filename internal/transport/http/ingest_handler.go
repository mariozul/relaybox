package http

import (
	"encoding/json"
	"net/http"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
)

// IngestHandler handles POST /v1/events.
type IngestHandler struct {
	svc *service.IngestService
}

func NewIngestHandler(svc *service.IngestService) *IngestHandler {
	return &IngestHandler{svc: svc}
}

type ingestRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	DedupKey  string          `json:"dedup_key"`
}

type ingestResponse struct {
	EventID     string `json:"event_id"`
	IsDuplicate bool   `json:"is_duplicate"`
}

func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := TenantID(r.Context())
	if !ok {
		http.Error(w, `{"error":"tenant identity required"}`, http.StatusForbidden)
		return
	}

	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.ErrInvalidArgument, http.StatusBadRequest)
		return
	}

	result, err := h.svc.IngestEvent(r.Context(), service.IngestEventRequest{
		TenantID:  tenantID,
		EventType: req.EventType,
		Payload:   []byte(req.Payload),
		DedupKey:  req.DedupKey,
		CreatedBy: tenantID,
	})
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case domain.ErrInvalidEventPayload, domain.ErrInvalidArgument:
			status = http.StatusBadRequest
		case domain.ErrTenantRequired:
			status = http.StatusForbidden
		}
		writeError(w, err, status)
		return
	}

	writeJSON(w, http.StatusOK, ingestResponse{
		EventID:     result.EventID,
		IsDuplicate: result.IsDuplicate,
	})
}

func writeError(w http.ResponseWriter, _ error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := http.StatusText(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
