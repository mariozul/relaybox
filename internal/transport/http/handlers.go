package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
)

// TenantCtxKey is the context key for tenant identity (RULE-SEC-01).
type ctxKey string

const TenantCtxKey ctxKey = "tenant_id"

// IngestHandler handles POST /v1/events.
type IngestHandler struct {
	svc    *service.EventService
	logger *slog.Logger
}

func NewIngestHandler(svc *service.EventService, logger *slog.Logger) *IngestHandler {
	return &IngestHandler{svc: svc, logger: logger}
}

func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := TenantIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tenant identity required", "UNAUTHORIZED")
		return
	}

	var req IngestEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}
	defer r.Body.Close()

	if req.EventType == "" || req.DedupKey == "" {
		writeError(w, http.StatusBadRequest, "event_type and dedup_key are required", "BAD_REQUEST")
		return
	}

	evt, err := h.svc.Ingest(r.Context(), tenantID, req.EventType, req.Payload, req.DedupKey)
	if err != nil {
		if err == domain.ErrConflict {
			writeError(w, http.StatusConflict, "event conflict", "CONFLICT")
			return
		}
		h.logger.Error("ingest failed", "err", err, "tenant_id", tenantID)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	h.logger.Info("event ingested", "event_id", evt.ID, "tenant_id", tenantID, "event_type", evt.EventType, "dedup_key", evt.DedupKey)
	writeJSON(w, http.StatusCreated, IngestEventResponse{
		ID:        evt.ID,
		TenantID:  evt.TenantID,
		EventType: evt.EventType,
		DedupKey:  evt.DedupKey,
	})
}

// SubscriptionHandler handles POST/GET/DELETE /v1/subscriptions.
type SubscriptionHandler struct {
	svc    *service.SubscriptionService
	logger *slog.Logger
}

func NewSubscriptionHandler(svc *service.SubscriptionService, logger *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, logger: logger}
}

func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := TenantIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tenant identity required", "UNAUTHORIZED")
		return
	}

	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}
	defer r.Body.Close()

	if req.EventType == "" || req.TargetURL == "" {
		writeError(w, http.StatusBadRequest, "event_type and target_url are required", "BAD_REQUEST")
		return
	}

	sub, err := h.svc.Create(r.Context(), tenantID, req.EventType, req.TargetURL)
	if err != nil {
		h.logger.Error("subscription create failed", "err", err, "tenant_id", tenantID)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	h.logger.Info("subscription created", "sub_id", sub.ID, "tenant_id", tenantID, "event_type", sub.EventType)
	writeJSON(w, http.StatusCreated, subscriptionToResponse(sub))
}

func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := TenantIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tenant identity required", "UNAUTHORIZED")
		return
	}

	subs, err := h.svc.List(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("subscription list failed", "err", err, "tenant_id", tenantID)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	resp := make([]SubscriptionResponse, 0, len(subs))
	for _, sub := range subs {
		resp = append(resp, subscriptionToResponse(sub))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	_, _ = TenantIDFromCtx(r.Context()) // validated in middleware

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "subscription id required", "BAD_REQUEST")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if err == domain.ErrNotFound {
			writeError(w, http.StatusNotFound, "subscription not found", "NOT_FOUND")
			return
		}
		h.logger.Error("subscription delete failed", "err", err, "sub_id", id)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

// TenantIDFromCtx extracts tenant ID from context (RULE-SEC-01).
func TenantIDFromCtx(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(TenantCtxKey).(string)
	return val, ok
}

func subscriptionToResponse(sub domain.Subscription) SubscriptionResponse {
	resp := SubscriptionResponse{
		ID:        sub.ID,
		TenantID:  sub.TenantID,
		EventType: sub.EventType,
		TargetURL: sub.TargetURL,
		CreatedAt: sub.CreatedAt.Format(time.RFC3339),
		CreatedBy: sub.CreatedBy,
	}
	if sub.UpdatedAt != nil {
		s := sub.UpdatedAt.Format(time.RFC3339)
		resp.UpdatedAt = &s
	}
	if sub.DeletedAt != nil {
		s := sub.DeletedAt.Format(time.RFC3339)
		resp.DeletedAt = &s
	}
	return resp
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, ErrorResponse{Error: msg, Code: code})
}
