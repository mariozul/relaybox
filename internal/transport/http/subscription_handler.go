package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
	"github.com/mariozul/relaybox/internal/transport/http/dto"
)

type SubscriptionHandler struct {
	svc    *service.SubscriptionService
	logger *slog.Logger
}

func NewSubscriptionHandler(svc *service.SubscriptionService, logger *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, logger: logger}
}

func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "TENANT_CONTEXT_MISSING", "tenant identity missing")
		return
	}
	var req dto.CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON")
		return
	}
	sub, err := h.svc.Create(r.Context(), tenantID, req.EventType, req.TargetURL, "system")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_EVENT_PAYLOAD", err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(subToResponse(sub))
}

func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "TENANT_CONTEXT_MISSING", "tenant identity missing")
		return
	}
	subs, err := h.svc.List(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list subscriptions")
		return
	}
	out := make([]dto.SubscriptionResponse, 0, len(subs))
	for _, s := range subs {
		out = append(out, subToResponse(s))
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "TENANT_CONTEXT_MISSING", "tenant identity missing")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "id is required")
		return
	}
	if err := h.svc.Delete(r.Context(), tenantID, id); err != nil {
		writeError(w, http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func subToResponse(s *domain.Subscription) dto.SubscriptionResponse {
	r := dto.SubscriptionResponse{
		ID: s.ID, TenantID: s.TenantID, EventType: s.EventType, TargetURL: s.TargetURL,
		CreatedAt: s.CreatedAt.Format(time.RFC3339), CreatedBy: s.CreatedBy,
	}
	if s.UpdatedAt != nil { v := s.UpdatedAt.Format(time.RFC3339); r.UpdatedAt = &v }
	if s.DeletedAt != nil  { v := s.DeletedAt.Format(time.RFC3339); r.DeletedAt = &v }
	r.UpdatedBy = s.UpdatedBy
	r.DeletedBy = s.DeletedBy
	return r
}
