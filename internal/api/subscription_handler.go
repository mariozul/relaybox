package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
)

// subManager is the interface SubscriptionHandler needs from SubscriptionService.
type subManager interface {
	Create(ctx context.Context, input service.CreateInput) (*domain.Subscription, error)
	List(ctx context.Context, tenantID string) ([]domain.Subscription, error)
	Delete(ctx context.Context, tenantID, id string) error
}

// SubscriptionHandler handles HTTP subscription CRUD.
type SubscriptionHandler struct {
	svc subManager
}

// NewSubscriptionHandler creates a SubscriptionHandler.
func NewSubscriptionHandler(svc subManager) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
}

// HandleCreate handles POST /v1/subscriptions.
func (h *SubscriptionHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	tenantID := GetTenantID(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "tenant identity required")
		return
	}

	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "cannot parse request body")
		return
	}

	sub, err := h.svc.Create(r.Context(), service.CreateInput{
		TenantID:  tenantID,
		EventType: req.EventType,
		TargetURL: req.TargetURL,
		CreatedBy: tenantID,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, SubscriptionResponse{
		ID:        sub.ID,
		EventType: sub.EventType,
		TargetURL: sub.TargetURL,
	})
}

// HandleList handles GET /v1/subscriptions.
func (h *SubscriptionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	tenantID := GetTenantID(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "tenant identity required")
		return
	}

	subs, err := h.svc.List(r.Context(), tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := make([]SubscriptionResponse, 0, len(subs))
	for _, s := range subs {
		resp = append(resp, SubscriptionResponse{
			ID:        s.ID,
			EventType: s.EventType,
			TargetURL: s.TargetURL,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleDelete handles DELETE /v1/subscriptions/{id}.
func (h *SubscriptionHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	tenantID := GetTenantID(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "tenant identity required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "subscription id required")
		return
	}

	if err := h.svc.Delete(r.Context(), tenantID, id); err != nil {
		writeDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
