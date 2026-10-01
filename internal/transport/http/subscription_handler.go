package http

import (
	"encoding/json"
	"net/http"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
)

// SubscriptionHandler handles CRUD /v1/subscriptions.
type SubscriptionHandler struct {
	svc *service.SubscriptionService
}

func NewSubscriptionHandler(svc *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
}

type createSubRequest struct {
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
}

type subResponse struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
}

func subToResponse(s domain.Subscription) subResponse {
	return subResponse{
		ID: s.ID, TenantID: s.TenantID, EventType: s.EventType, TargetURL: s.TargetURL,
	}
}

func (h *SubscriptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := TenantID(r.Context())
	if !ok {
		writeError(w, domain.ErrTenantRequired, http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.create(w, r, tenantID)
	case http.MethodGet:
		h.list(w, r, tenantID)
	case http.MethodDelete:
		h.delete(w, r, tenantID)
	default:
		writeError(w, domain.ErrInvalidArgument, http.StatusMethodNotAllowed)
	}
}

func (h *SubscriptionHandler) create(w http.ResponseWriter, r *http.Request, tenantID string) {
	var req createSubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	sub, err := h.svc.CreateSubscription(r.Context(), service.CreateSubscriptionRequest{
		TenantID: tenantID, EventType: req.EventType, TargetURL: req.TargetURL,
		CreatedBy: tenantID,
	})
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case err == domain.ErrInvalidArgument:
			status = http.StatusBadRequest
		case err == domain.ErrTenantRequired:
			status = http.StatusForbidden
		}
		writeError(w, err, status)
		return
	}
	writeJSON(w, http.StatusCreated, subToResponse(*sub))
}

func (h *SubscriptionHandler) list(w http.ResponseWriter, r *http.Request, tenantID string) {
	subs, err := h.svc.ListSubscriptions(r.Context(), tenantID)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	out := make([]subResponse, 0, len(subs))
	for _, s := range subs {
		out = append(out, subToResponse(s))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *SubscriptionHandler) delete(w http.ResponseWriter, r *http.Request, tenantID string) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, domain.ErrInvalidArgument, http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteSubscription(r.Context(), tenantID, id); err != nil {
		if err == domain.ErrNotFound {
			writeError(w, err, http.StatusNotFound)
			return
		}
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
