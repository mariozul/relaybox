package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	httpsrv "github.com/mariozul/relaybox/internal/transport/http"
	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

// stubs that implement all domain port interfaces needed by service layer

type stubEventStore struct {
	createResult domain.Event
	createErr    error
	dedupResult  domain.Event
	dedupErr     error
}

func (s *stubEventStore) CreateEvent(ctx context.Context, tenantID, eventType string, payload []byte, dedupKey string) (domain.Event, error) {
	return s.createResult, s.createErr
}
func (s *stubEventStore) GetEvent(ctx context.Context, id string) (domain.Event, error) {
	return domain.Event{}, domain.ErrNotFound
}
func (s *stubEventStore) GetEventByDedup(ctx context.Context, tenantID, dedupKey string) (domain.Event, error) {
	return s.dedupResult, s.dedupErr
}

type stubOutboxStore struct{}

func (s *stubOutboxStore) CreateOutbox(ctx context.Context, eventID, subscriptionID string) (domain.Outbox, error) {
	return domain.Outbox{}, nil
}
func (s *stubOutboxStore) FetchPending(ctx context.Context, limit int) ([]domain.Outbox, error) {
	return nil, nil
}
func (s *stubOutboxStore) MarkDelivered(ctx context.Context, id string) error { return nil }
func (s *stubOutboxStore) MarkFailed(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error {
	return nil
}
func (s *stubOutboxStore) MarkDeadLetter(ctx context.Context, id string, errMsg string) error { return nil }

type stubSubscriptionStore struct{}

func (s *stubSubscriptionStore) CreateSubscription(ctx context.Context, tenantID, eventType, targetURL string) (domain.Subscription, error) {
	return domain.Subscription{}, nil
}
func (s *stubSubscriptionStore) GetSubscription(ctx context.Context, id string) (domain.Subscription, error) {
	return domain.Subscription{}, domain.ErrNotFound
}
func (s *stubSubscriptionStore) GetSubscriptionsByEventType(ctx context.Context, tenantID, eventType string) ([]domain.Subscription, error) {
	return nil, nil
}
func (s *stubSubscriptionStore) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return nil, nil
}
func (s *stubSubscriptionStore) DeleteSubscription(ctx context.Context, id string) error { return nil }

type stubTxManager struct{}

func (s *stubTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestIngestHandler_Success(t *testing.T) {
	t.Parallel()

	store := &stubEventStore{
		createResult: domain.Event{
			ID:        "evt_123",
			TenantID:  "tenant_a",
			EventType: "order.created",
			Payload:   []byte(`{"id":"123"}`),
			DedupKey:  "dedup_1",
		},
		dedupErr: domain.ErrNotFound,
	}
	svc := service.NewEventService(store, &stubOutboxStore{}, &stubSubscriptionStore{}, &stubTxManager{})
	handler := httpsrv.NewIngestHandler(svc, newTestLogger())

	body := bytes.NewBufferString(`{"event_type":"order.created","payload":{"id":"123"},"dedup_key":"dedup_1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	req = req.WithContext(context.WithValue(req.Context(), httpsrv.TenantCtxKey, "tenant_a"))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIngestHandler_Unauthorized(t *testing.T) {
	t.Parallel()

	store := &stubEventStore{}
	svc := service.NewEventService(store, &stubOutboxStore{}, &stubSubscriptionStore{}, &stubTxManager{})
	handler := httpsrv.NewIngestHandler(svc, newTestLogger())

	body := bytes.NewBufferString(`{"event_type":"order.created","payload":{},"dedup_key":"dedup_1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	// No tenant context

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestIngestHandler_BadRequest(t *testing.T) {
	t.Parallel()

	store := &stubEventStore{dedupErr: domain.ErrNotFound}
	svc := service.NewEventService(store, &stubOutboxStore{}, &stubSubscriptionStore{}, &stubTxManager{})
	handler := httpsrv.NewIngestHandler(svc, newTestLogger())

	body := bytes.NewBufferString(`{"payload":{}}`) // missing required fields
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	req = req.WithContext(context.WithValue(req.Context(), httpsrv.TenantCtxKey, "tenant_a"))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestTenantMiddleware(t *testing.T) {
	t.Parallel()

	var capturedTenant string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, _ := httpsrv.TenantIDFromCtx(r.Context())
		capturedTenant = tenant
		w.WriteHeader(http.StatusOK)
	})

	middleware := httpsrv.TenantMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant_xyz")
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if capturedTenant != "tenant_xyz" {
		t.Errorf("expected tenant_xyz in context, got %s", capturedTenant)
	}
}

func TestTenantMiddleware_NoHeader(t *testing.T) {
	t.Parallel()

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := httpsrv.TenantIDFromCtx(r.Context())
		if ok || tenant != "" {
			t.Errorf("expected no tenant, got %q", tenant)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := httpsrv.TenantMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestSubscriptionResponse_Formatting(t *testing.T) {
	t.Parallel()
	resp := httpsrv.SubscriptionResponse{
		ID:        "sub_1",
		TenantID:  "t1",
		EventType: "order.created",
		TargetURL: "https://example.com",
		CreatedAt: "2026-10-01T00:00:00Z",
		CreatedBy: "api",
	}

	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["id"] != "sub_1" {
		t.Errorf("expected sub_1, got %v", parsed["id"])
	}
}

func TestErrorResponse(t *testing.T) {
	t.Parallel()
	resp := httpsrv.ErrorResponse{Error: "bad request", Code: "BAD_REQUEST"}
	body, _ := json.Marshal(resp)
	if len(body) == 0 {
		t.Fatal("empty error response")
	}
}
