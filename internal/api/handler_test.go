package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
)

// stubEventIngester for handler tests.
type stubEventIngester struct {
	ingestFn func(ctx context.Context, input service.IngestInput) (*service.IngestResult, error)
}

func (s *stubEventIngester) Ingest(ctx context.Context, input service.IngestInput) (*service.IngestResult, error) {
	return s.ingestFn(ctx, input)
}

// stubSubManager for handler tests.
type stubSubManager struct {
	createFn func(ctx context.Context, input service.CreateInput) (*domain.Subscription, error)
	listFn   func(ctx context.Context, tenantID string) ([]domain.Subscription, error)
	deleteFn func(ctx context.Context, tenantID, id string) error
}

func (s *stubSubManager) Create(ctx context.Context, input service.CreateInput) (*domain.Subscription, error) {
	return s.createFn(ctx, input)
}
func (s *stubSubManager) List(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return s.listFn(ctx, tenantID)
}
func (s *stubSubManager) Delete(ctx context.Context, tenantID, id string) error {
	return s.deleteFn(ctx, tenantID, id)
}

// withTenant adds the X-Tenant-ID to a request context and header.
func withTenant(r *http.Request, tenantID string) *http.Request {
	ctx := context.WithValue(r.Context(), tenantIDKey, tenantID)
	return r.WithContext(ctx)
}

func TestEventHandler_RequiresTenantHeader(t *testing.T) {
	t.Parallel()
	h := NewEventHandler(&stubEventIngester{})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.HandleCreateEvent(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without X-Tenant-ID, got %d", w.Code)
	}
}

func TestEventHandler_CreateEvent_Success(t *testing.T) {
	t.Parallel()
	h := NewEventHandler(&stubEventIngester{
		ingestFn: func(ctx context.Context, input service.IngestInput) (*service.IngestResult, error) {
			return &service.IngestResult{EventID: "evt-1", OutboxRowCount: 1}, nil
		},
	})

	body := `{"event_type":"order.created","payload":"eyJvcmRlciI6MX0=","dedup_key":"dk-1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(body))
	req = withTenant(req, "tenant-a")
	w := httptest.NewRecorder()

	h.HandleCreateEvent(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp CreateEventResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.EventID != "evt-1" {
		t.Fatalf("expected evt-1, got %s", resp.EventID)
	}
}

func TestEventHandler_ReturnsBadRequestOnInvalidJSON(t *testing.T) {
	t.Parallel()
	h := NewEventHandler(&stubEventIngester{})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`not json`))
	req = withTenant(req, "tenant-a")
	w := httptest.NewRecorder()
	h.HandleCreateEvent(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubscriptionHandler_RequiresTenantHeader(t *testing.T) {
	t.Parallel()
	h := NewSubscriptionHandler(&stubSubManager{})
	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSubscriptionHandler_CreateAndList(t *testing.T) {
	t.Parallel()
	h := NewSubscriptionHandler(&stubSubManager{
		createFn: func(ctx context.Context, input service.CreateInput) (*domain.Subscription, error) {
			return &domain.Subscription{ID: "sub-1", EventType: input.EventType, TargetURL: input.TargetURL}, nil
		},
		listFn: func(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
			return []domain.Subscription{{ID: "sub-1", EventType: "order.created", TargetURL: "https://example.com/hook"}}, nil
		},
	})

	body := `{"event_type":"order.created","target_url":"https://example.com/hook"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", strings.NewReader(body))
	req = withTenant(req, "tenant-a")
	w := httptest.NewRecorder()
	h.HandleCreate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req2 = withTenant(req2, "tenant-a")
	w2 := httptest.NewRecorder()
	h.HandleList(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
}

func TestSubscriptionHandler_Delete(t *testing.T) {
	t.Parallel()
	h := NewSubscriptionHandler(&stubSubManager{
		deleteFn: func(ctx context.Context, tenantID, id string) error {
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/sub-1", nil)
	req = withTenant(req, "tenant-a")
	req.SetPathValue("id", "sub-1")
	w := httptest.NewRecorder()
	h.HandleDelete(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestMiddleware_ExtractsTenantID(t *testing.T) {
	t.Parallel()
	var captured string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetTenantID(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := TenantMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-z")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if captured != "tenant-z" {
		t.Fatalf("expected tenant-z, got %s", captured)
	}
}
