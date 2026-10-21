// Package handler implements HTTP transport handlers.
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

// SubscriptionHandler defines HTTP handlers for subscription CRUD operations.
type SubscriptionHandler interface {
	CreateSubscriptionHandler(w http.ResponseWriter, r *http.Request)
	GetSubscriptionHandler(w http.ResponseWriter, r *http.Request)
	ListSubscriptionsHandler(w http.ResponseWriter, r *http.Request)
	UpdateSubscriptionHandler(w http.ResponseWriter, r *http.Request)
	DeleteSubscriptionHandler(w http.ResponseWriter, r *http.Request)
}

type subscriptionHandler struct {
	subscriptionService service.SubscriptionService
}

// NewSubscriptionHandler creates a new SubscriptionHandler.
func NewSubscriptionHandler(subscriptionService service.SubscriptionService) SubscriptionHandler {
	return &subscriptionHandler{subscriptionService: subscriptionService}
}

// CreateSubscriptionHandler handles POST /v1/subscriptions (FR-SUB-01).
func (h *subscriptionHandler) CreateSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized: missing tenant identity")
		return
	}

	var req dto.CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, "invalid JSON request body")
		return
	}

	subID, err := h.subscriptionService.CreateSubscription(r.Context(), tenantID, &req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, err.Error())
			return
		case errors.Is(err, domain.ErrUnauthorized):
			writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized access")
			return
		default:
			slog.ErrorContext(r.Context(), "create subscription failed",
				"tenant_id", tenantID,
				"error", err.Error(),
			)
			writeError(w, http.StatusInternalServerError, domain.ErrCodeInternal, "internal server error")
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"subscription_id": subID,
	})
}

// GetSubscriptionHandler handles GET /v1/subscriptions/:id (FR-SUB-01, FR-SUB-02).
func (h *subscriptionHandler) GetSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized: missing tenant identity")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, "subscription ID is required")
		return
	}

	sub, err := h.subscriptionService.GetSubscription(r.Context(), tenantID, id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSubscriptionNotFound):
			writeError(w, http.StatusNotFound, domain.ErrCodeSubscriptionNotFound, "subscription not found")
			return
		case errors.Is(err, domain.ErrUnauthorized):
			writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized access")
			return
		default:
			slog.ErrorContext(r.Context(), "get subscription failed",
				"tenant_id", tenantID,
				"subscription_id", id,
				"error", err.Error(),
			)
			writeError(w, http.StatusInternalServerError, domain.ErrCodeInternal, "internal server error")
			return
		}
	}

	resp := subscriptionToResponse(sub)
	writeJSON(w, http.StatusOK, resp)
}

// ListSubscriptionsHandler handles GET /v1/subscriptions (FR-SUB-01, FR-SUB-02).
func (h *subscriptionHandler) ListSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized: missing tenant identity")
		return
	}

	subs, err := h.subscriptionService.ListSubscriptions(r.Context(), tenantID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized access")
			return
		default:
			slog.ErrorContext(r.Context(), "list subscriptions failed",
				"tenant_id", tenantID,
				"error", err.Error(),
			)
			writeError(w, http.StatusInternalServerError, domain.ErrCodeInternal, "internal server error")
			return
		}
	}

	resp := dto.SubscriptionListResponse{
		Subscriptions: make([]dto.SubscriptionResponse, 0, len(subs)),
	}
	for _, sub := range subs {
		resp.Subscriptions = append(resp.Subscriptions, subscriptionToResponse(&sub))
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpdateSubscriptionHandler handles PUT /v1/subscriptions/:id (FR-SUB-01, FR-SUB-02).
func (h *subscriptionHandler) UpdateSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized: missing tenant identity")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, "subscription ID is required")
		return
	}

	var req dto.UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, "invalid JSON request body")
		return
	}

	err := h.subscriptionService.UpdateSubscription(r.Context(), tenantID, id, &req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSubscriptionNotFound):
			writeError(w, http.StatusNotFound, domain.ErrCodeSubscriptionNotFound, "subscription not found")
			return
		case errors.Is(err, domain.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, err.Error())
			return
		case errors.Is(err, domain.ErrUnauthorized):
			writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized access")
			return
		default:
			slog.ErrorContext(r.Context(), "update subscription failed",
				"tenant_id", tenantID,
				"subscription_id", id,
				"error", err.Error(),
			)
			writeError(w, http.StatusInternalServerError, domain.ErrCodeInternal, "internal server error")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"updated"}`))
}

// DeleteSubscriptionHandler handles DELETE /v1/subscriptions/:id (FR-SUB-01, FR-SUB-02).
func (h *subscriptionHandler) DeleteSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	if tenantID == "" {
		writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized: missing tenant identity")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, "subscription ID is required")
		return
	}

	err := h.subscriptionService.DeleteSubscription(r.Context(), tenantID, id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSubscriptionNotFound):
			writeError(w, http.StatusNotFound, domain.ErrCodeSubscriptionNotFound, "subscription not found")
			return
		case errors.Is(err, domain.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, domain.ErrCodeInvalidInput, err.Error())
			return
		case errors.Is(err, domain.ErrUnauthorized):
			writeError(w, http.StatusForbidden, domain.ErrCodeUnauthorized, "unauthorized access")
			return
		default:
			slog.ErrorContext(r.Context(), "delete subscription failed",
				"tenant_id", tenantID,
				"subscription_id", id,
				"error", err.Error(),
			)
			writeError(w, http.StatusInternalServerError, domain.ErrCodeInternal, "internal server error")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// subscriptionToResponse converts a domain.Subscription to a SubscriptionResponse DTO.
func subscriptionToResponse(sub *domain.Subscription) dto.SubscriptionResponse {
	return dto.SubscriptionResponse{
		ID:        sub.ID,
		TenantID:  sub.TenantID,
		EventType: sub.EventType,
		TargetURL: sub.TargetURL,
		CreatedAt: sub.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy: sub.CreatedBy,
		UpdatedAt: sub.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedBy: sub.UpdatedBy,
	}
}
