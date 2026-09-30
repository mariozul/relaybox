package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/tenant"
)

type API struct{ store domain.Store }

func New(store domain.Store, authToken string) http.Handler {
	a := &API{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/events", a.ingest)
	mux.HandleFunc("POST /v1/subscriptions", a.createSubscription)
	mux.HandleFunc("GET /v1/subscriptions", a.listSubscriptions)
	mux.HandleFunc("DELETE /v1/subscriptions/{id}", a.deleteSubscription)
	return TenantAuth(authToken, mux)
}

func TenantAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := strings.TrimSpace(r.Header.Get("X-Relaybox-Tenant-Id"))
		if token == "" || r.Header.Get("Authorization") != "Bearer "+token || tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenantID)))
	})
}

func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EventType string          `json:"event_type"`
		Payload   json.RawMessage `json:"payload"`
		DedupKey  string          `json:"dedup_key"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	event, err := domain.NewEvent(input.EventType, input.Payload, input.DedupKey)
	if err != nil {
		writeError(w, err)
		return
	}
	event.CreatedBy, _ = tenant.Require(r.Context())
	event, created, err := a.store.Ingest(r.Context(), event)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]string{"id": event.ID})
}

func (a *API) createSubscription(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EventType string `json:"event_type"`
		TargetURL string `json:"target_url"`
	}
	if decode(w, r, &input) != nil {
		return
	}
	sub, err := domain.NewSubscription(input.EventType, input.TargetURL)
	if err != nil {
		writeError(w, err)
		return
	}
	sub, err = a.store.CreateSubscription(r.Context(), sub)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

func (a *API) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListSubscriptions(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteSubscription(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrInvalid) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
