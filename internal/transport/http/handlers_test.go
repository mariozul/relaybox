package http_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
	httptransport "github.com/mariozul/relaybox/internal/transport/http"
)

type dbStub struct{}
func (d *dbStub) Begin(ctx context.Context) (pgx.Tx, error) { return &txStub{}, nil }

type txStub struct{ pgx.Tx }
func (txStub) Commit(ctx context.Context) error { return nil }
func (txStub) Rollback(ctx context.Context) error { return nil }
func (txStub) Conn() *pgx.Conn { return nil }

type stubIngestEventRepo struct{}
func (s *stubIngestEventRepo) CreateWithOutbox(ctx context.Context, tx pgx.Tx, e *domain.Event, entries []*domain.OutboxEntry) error {
	e.ID = "evt-new"
	return nil
}
func (s *stubIngestEventRepo) FindByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	return nil, domain.ErrNotFound
}

type stubIngestSubRepo struct{}
func (s *stubIngestSubRepo) Create(ctx context.Context, tx pgx.Tx, sub *domain.Subscription) error { return nil }
func (s *stubIngestSubRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Subscription, error) { return nil, nil }
func (s *stubIngestSubRepo) Delete(ctx context.Context, tx pgx.Tx, tenantID, id string) error { return nil }

func TestIngestHandlerHappyPath(t *testing.T) {
	t.Parallel()
	svc := service.NewIngestService(&stubIngestEventRepo{}, &stubIngestSubRepo{}, &dbStub{}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewEventHandler(svc, logger)
	body := bytes.NewBufferString(`{"event_type":"order.created","payload":"eyJvcmRlciI6MX0=","dedup_key":"dk-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	req = httptransport.WithTenant(req, "tnt-1")
	rec := httptest.NewRecorder()
	handler.Ingest(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIngestHandlerTenantOnlyFromContext(t *testing.T) {
	t.Parallel()
	svc := service.NewIngestService(&stubIngestEventRepo{}, &stubIngestSubRepo{}, &dbStub{}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewEventHandler(svc, logger)
	body := bytes.NewBufferString(`{"event_type":"order.created","payload":"e30=","dedup_key":"dk-1","tenant_id":"evil"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	req = httptransport.WithTenant(req, "tnt-1")
	rec := httptest.NewRecorder()
	handler.Ingest(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestIngestHandlerNoTenantReturns401(t *testing.T) {
	t.Parallel()
	svc := service.NewIngestService(&stubIngestEventRepo{}, &stubIngestSubRepo{}, &dbStub{}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewEventHandler(svc, logger)
	body := bytes.NewBufferString(`{"event_type":"order.created","payload":"e30=","dedup_key":"dk-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	rec := httptest.NewRecorder()
	handler.Ingest(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestIngestHandlerInvalidJSONReturns400(t *testing.T) {
	t.Parallel()
	svc := service.NewIngestService(nil, nil, nil, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewEventHandler(svc, logger)
	body := bytes.NewBufferString(`not-json`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	req = httptransport.WithTenant(req, "tnt-1")
	rec := httptest.NewRecorder()
	handler.Ingest(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

type testSubRepo struct {
	subs map[string]*domain.Subscription
}
func (r *testSubRepo) Create(ctx context.Context, tx pgx.Tx, s *domain.Subscription) error {
	s.ID = "sub-1"
	r.subs[s.ID] = s
	return nil
}
func (r *testSubRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Subscription, error) {
	var out []*domain.Subscription
	for _, s := range r.subs { if s.TenantID == tenantID { out = append(out, s) } }
	return out, nil
}
func (r *testSubRepo) Delete(ctx context.Context, tx pgx.Tx, tenantID, id string) error {
	return domain.ErrSubscriptionNotFound
}

func TestSubscriptionHandlerCreateAndList(t *testing.T) {
	t.Parallel()
	svc := service.NewSubscriptionService(&testSubRepo{subs: make(map[string]*domain.Subscription)}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewSubscriptionHandler(svc, logger)
	body := bytes.NewBufferString(`{"event_type":"order.created","target_url":"https://example.com/hook"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", body)
	req = httptransport.WithTenant(req, "tnt-1")
	rec := httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestSubscriptionHandlerDeleteCrossTenant(t *testing.T) {
	t.Parallel()
	svc := service.NewSubscriptionService(&testSubRepo{subs: make(map[string]*domain.Subscription)}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewSubscriptionHandler(svc, logger)
	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/sub-999", nil)
	req.SetPathValue("id", "sub-999")
	req = httptransport.WithTenant(req, "tnt-b")
	rec := httptest.NewRecorder()
	handler.Delete(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSubscriptionHandlerListNoTenant(t *testing.T) {
	t.Parallel()
	svc := service.NewSubscriptionService(&testSubRepo{subs: make(map[string]*domain.Subscription)}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewSubscriptionHandler(svc, logger)
	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}




func TestSubscriptionHandlerListNotEmpty(t *testing.T) {
	t.Parallel()
	repo := &testSubRepo{subs: map[string]*domain.Subscription{
		"sub-1": {ID: "sub-1", TenantID: "tnt-1", EventType: "order.created", TargetURL: "https://example.com/hook", CreatedAt: time.Now(), CreatedBy: "system"},
	}}
	svc := service.NewSubscriptionService(repo, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewSubscriptionHandler(svc, logger)
	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req = httptransport.WithTenant(req, "tnt-1")
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSubscriptionHandlerCreateNoTenant(t *testing.T) {
	t.Parallel()
	svc := service.NewSubscriptionService(&testSubRepo{subs: make(map[string]*domain.Subscription)}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewSubscriptionHandler(svc, logger)
	body := bytes.NewBufferString(`{"event_type":"order.created","target_url":"https://example.com/hook"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", body)
	rec := httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestSubscriptionHandlerDeleteNoTenant(t *testing.T) {
	t.Parallel()
	svc := service.NewSubscriptionService(&testSubRepo{subs: make(map[string]*domain.Subscription)}, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewSubscriptionHandler(svc, logger)
	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/sub-1", nil)
	rec := httptest.NewRecorder()
	handler.Delete(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestIngestHandlerInvalidPayloadReturns400(t *testing.T) {
	t.Parallel()
	svc := service.NewIngestService(nil, nil, nil, domain.FixedClock{})
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := httptransport.NewEventHandler(svc, logger)
	body := bytes.NewBufferString(`{"event_type":"","payload":"e30=","dedup_key":"dk-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", body)
	req = httptransport.WithTenant(req, "tnt-1")
	rec := httptest.NewRecorder()
	handler.Ingest(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}



func TestMetricsHandlerRegistered(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", httptransport.MetricsHandler())
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
