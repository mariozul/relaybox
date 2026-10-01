package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
	"github.com/mariozul/relaybox/internal/transport/http/dto"
)

type EventHandler struct {
	ingest *service.IngestService
	logger *slog.Logger
}

func NewEventHandler(ingest *service.IngestService, logger *slog.Logger) *EventHandler {
	return &EventHandler{ingest: ingest, logger: logger}
}

func (h *EventHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantFromContext(r.Context())
	if tenantID == "" {
		h.logger.WarnContext(r.Context(), "ingest: tenant missing from context")
		writeError(w, http.StatusUnauthorized, "TENANT_CONTEXT_MISSING", "tenant identity missing")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ingest: read body", "err", err)
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "cannot read request body")
		return
	}
	defer r.Body.Close()

	var req dto.IngestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.logger.WarnContext(r.Context(), "ingest: invalid JSON", "err", err)
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON")
		return
	}

	evt, err := h.ingest.Ingest(r.Context(), tenantID, req.EventType, req.Payload, req.DedupKey)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidEventPayload) {
			writeError(w, http.StatusBadRequest, "INVALID_EVENT_PAYLOAD", err.Error())
			return
		}
		if errors.Is(err, domain.ErrTenantMissing) {
			writeError(w, http.StatusUnauthorized, "TENANT_CONTEXT_MISSING", err.Error())
			return
		}
		h.logger.ErrorContext(r.Context(), "ingest: internal error", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		return
	}

	status := http.StatusCreated
	resp := dto.IngestResponse{EventID: evt.ID}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

const maxBodySize = 256*1024 + 1024

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": msg})
}
