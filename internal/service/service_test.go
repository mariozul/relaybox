package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/infrastructure/httpclient"
	"github.com/mariozul/relaybox/internal/service"
)

// ---- Shared Stubs ----

type stubTx struct{ pgx.Tx }
func (s stubTx) Commit(ctx context.Context) error  { return nil }
func (s stubTx) Rollback(ctx context.Context) error { return nil }
func (s stubTx) Conn() *pgx.Conn                    { return nil }

type stubDBPool struct {
	beginFn func(ctx context.Context) (pgx.Tx, error)
}
func (s *stubDBPool) Begin(ctx context.Context) (pgx.Tx, error) {
	if s.beginFn != nil { return s.beginFn(ctx) }
	return &stubTx{}, nil
}
func testDB() *stubDBPool { return &stubDBPool{} }

type stubEventRepo struct {
	createWithOutboxFn func(ctx context.Context, tx pgx.Tx, e *domain.Event, entries []*domain.OutboxEntry) error
	findByDedupKeyFn   func(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error)
}
func (s *stubEventRepo) CreateWithOutbox(ctx context.Context, tx pgx.Tx, e *domain.Event, entries []*domain.OutboxEntry) error {
	return s.createWithOutboxFn(ctx, tx, e, entries)
}
func (s *stubEventRepo) FindByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	return s.findByDedupKeyFn(ctx, tenantID, dedupKey)
}

type stubSubRepo struct {
	createFn func(ctx context.Context, tx pgx.Tx, s *domain.Subscription) error
	listFn   func(ctx context.Context, tenantID string) ([]*domain.Subscription, error)
	deleteFn func(ctx context.Context, tx pgx.Tx, tenantID, id string) error
}
func (s *stubSubRepo) Create(ctx context.Context, tx pgx.Tx, sub *domain.Subscription) error {
	if s.createFn != nil { return s.createFn(ctx, tx, sub) }
	return nil
}
func (s *stubSubRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Subscription, error) {
	if s.listFn != nil { return s.listFn(ctx, tenantID) }
	return nil, nil
}
func (s *stubSubRepo) Delete(ctx context.Context, tx pgx.Tx, tenantID, id string) error {
	if s.deleteFn != nil { return s.deleteFn(ctx, tx, tenantID, id) }
	return nil
}

type stubOutboxRepo struct {
	claimFn          func(ctx context.Context, limit int) ([]*domain.OutboxEntry, error)
	markDeliveredFn  func(ctx context.Context, id string) error
	markAttemptFn    func(ctx context.Context, id string, nextAt time.Time) error
	deadLetterFn     func(ctx context.Context, id string) error
}
func (s *stubOutboxRepo) ClaimPending(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
	if s.claimFn != nil { return s.claimFn(ctx, limit) }
	return nil, nil
}
func (s *stubOutboxRepo) MarkDelivered(ctx context.Context, id string) error {
	if s.markDeliveredFn != nil { return s.markDeliveredFn(ctx, id) }
	return nil
}
func (s *stubOutboxRepo) MarkAttempt(ctx context.Context, id string, nextAt time.Time) error {
	if s.markAttemptFn != nil { return s.markAttemptFn(ctx, id, nextAt) }
	return nil
}
func (s *stubOutboxRepo) DeadLetter(ctx context.Context, id string) error {
	if s.deadLetterFn != nil { return s.deadLetterFn(ctx, id) }
	return nil
}

// ---- Ingest Tests ----

func TestIngestInvalidPayload(t *testing.T) {
	t.Parallel()
	svc := service.NewIngestService(nil, nil, testDB(), domain.FixedClock{})
	_, err := svc.Ingest(context.Background(), "tnt-1", "", nil, "dk-1")
	if !errors.Is(err, domain.ErrInvalidEventPayload) {
		t.Fatalf("expected ErrInvalidEventPayload, got %v", err)
	}
}

func TestIngestMissingTenant(t *testing.T) {
	t.Parallel()
	svc := service.NewIngestService(nil, nil, testDB(), domain.FixedClock{})
	_, err := svc.Ingest(context.Background(), "", "order.created", []byte("{}"), "dk-1")
	if !errors.Is(err, domain.ErrTenantMissing) {
		t.Fatalf("expected ErrTenantMissing, got %v", err)
	}
}

func TestIngestDedupReturnsExisting(t *testing.T) {
	t.Parallel()
	existing := &domain.Event{ID: "evt-existing", TenantID: "tnt-1", EventType: "order.created"}
	repo := &stubEventRepo{
		createWithOutboxFn: func(ctx context.Context, tx pgx.Tx, e *domain.Event, entries []*domain.OutboxEntry) error {
			return &pgconn.PgError{Code: "23505"}
		},
		findByDedupKeyFn: func(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
			return existing, nil
		},
	}
	svc := service.NewIngestService(repo, nil, testDB(), domain.FixedClock{})
	evt, err := svc.Ingest(context.Background(), "tnt-1", "order.created", []byte("{}"), "dk-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.ID != "evt-existing" {
		t.Fatalf("dedup must return existing id, got %s", evt.ID)
	}
}

func TestIngestHappyPath(t *testing.T) {
	t.Parallel()
	repo := &stubEventRepo{
		createWithOutboxFn: func(ctx context.Context, tx pgx.Tx, e *domain.Event, entries []*domain.OutboxEntry) error {
			e.ID = uuid.New().String()
			return nil
		},
		findByDedupKeyFn: func(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
			return nil, domain.ErrNotFound
		},
	}
	svc := service.NewIngestService(repo, nil, testDB(), domain.FixedClock{})
	evt, err := svc.Ingest(context.Background(), "tnt-1", "order.created", []byte("{}"), "dk-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.ID == "" {
		t.Fatal("new event must have non-empty ID")
	}
}

func TestIngestDBDown(t *testing.T) {
	t.Parallel()
	db := &stubDBPool{beginFn: func(ctx context.Context) (pgx.Tx, error) { return nil, errors.New("db down") }}
	svc := service.NewIngestService(nil, nil, db, domain.FixedClock{})
	_, err := svc.Ingest(context.Background(), "tnt-1", "order.created", []byte("{}"), "dk-1")
	if err == nil {
		t.Fatal("expected error when DB down")
	}
}

// ---- Subscription Tests ----

func TestSubscriptionCreateHappyPath(t *testing.T) {
	t.Parallel()
	repo := &stubSubRepo{createFn: func(ctx context.Context, tx pgx.Tx, s *domain.Subscription) error { s.ID = "sub-1"; return nil }}
	svc := service.NewSubscriptionService(repo, domain.FixedClock{})
	sub, err := svc.Create(context.Background(), "tnt-1", "order.created", "https://example.com/hook", "system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ID != "sub-1" {
		t.Fatalf("expected sub-1, got %s", sub.ID)
	}
}

func TestSubscriptionListByTenant(t *testing.T) {
	t.Parallel()
	repo := &stubSubRepo{listFn: func(ctx context.Context, tenantID string) ([]*domain.Subscription, error) {
		return []*domain.Subscription{{ID: "sub-1", TenantID: tenantID}}, nil
	}}
	svc := service.NewSubscriptionService(repo, domain.FixedClock{})
	subs, err := svc.List(context.Background(), "tnt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(subs))
	}
}

func TestSubscriptionDeleteHappyPath(t *testing.T) {
	t.Parallel()
	repo := &stubSubRepo{deleteFn: func(ctx context.Context, tx pgx.Tx, tenantID, id string) error { return nil }}
	svc := service.NewSubscriptionService(repo, domain.FixedClock{})
	if err := svc.Delete(context.Background(), "tnt-1", "sub-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubscriptionDeleteCrossTenant(t *testing.T) {
	t.Parallel()
	repo := &stubSubRepo{deleteFn: func(ctx context.Context, tx pgx.Tx, tenantID, id string) error {
		return domain.ErrSubscriptionNotFound
	}}
	svc := service.NewSubscriptionService(repo, domain.FixedClock{})
	err := svc.Delete(context.Background(), "tnt-b", "sub-a")
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// ---- Dispatcher Tests ----

func TestDispatcherShutdownWaitsForWorkers(t *testing.T) {
	t.Parallel()
	repo := &stubOutboxRepo{claimFn: func(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	client := httpclient.New(30 * time.Second)
	policy := domain.DefaultDeliveryPolicy{MaxAttempts: 5}
	disp := service.NewDispatcher(repo, nil, client, policy, 2, 1*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = disp.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatcher did not shut down")
	}
}

func TestDispatcherLifecycle(t *testing.T) {
	t.Parallel()
	repo := &stubOutboxRepo{claimFn: func(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
		<-ctx.Done(); return nil, ctx.Err()
	}}
	client := httpclient.New(30 * time.Second)
	policy := domain.DefaultDeliveryPolicy{MaxAttempts: 5}
	disp := service.NewDispatcher(repo, nil, client, policy, 1, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan struct{})
	go func() { _ = disp.Run(ctx); close(ch) }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-ch
}

func TestDispatchProcessBatchWithTestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var delivered atomic.Bool
	repo := &stubOutboxRepo{
		claimFn: func(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
			return []*domain.OutboxEntry{{
				ID: "obx-ok", EventID: "evt-ok", SubscriptionID: "sub-ok",
				Status: domain.DeliveryStatusPending, Attempts: 0,
				NextAttemptAt: time.Now(), DeliveryID: "dlv-ok",
			}}, nil
		},
		markDeliveredFn: func(ctx context.Context, id string) error {
			delivered.Store(true)
			return nil
		},
	}
	client := httpclient.New(30 * time.Second)
	policy := domain.DefaultDeliveryPolicy{MaxAttempts: 5}
	disp := service.NewDispatcher(repo, nil, client, policy, 1, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch := make(chan struct{})
	go func() { _ = disp.Run(ctx); close(ch) }()
	<-ch
}

