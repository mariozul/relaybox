package service

import (
	"context"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
	
)

// --- Test doubles ---

type fakeEventRepo struct {
	store     map[string]*domain.Event
	byDedup   map[string]*domain.Event
	createFn  func(ctx context.Context, e *domain.Event) (*domain.Event, error)
}

func newFakeEventRepo() *fakeEventRepo {
	return &fakeEventRepo{
		store:   map[string]*domain.Event{},
		byDedup: map[string]*domain.Event{},
	}
}
func (r *fakeEventRepo) Create(ctx context.Context, e *domain.Event) (*domain.Event, error) {
	if r.createFn != nil {
		return r.createFn(ctx, e)
	}
	k := e.TenantID + "/" + e.DedupKey
	if ex, ok := r.byDedup[k]; ok {
		return ex, domain.ErrConflict
	}
	e.ID = "ev-" + e.DedupKey
	r.store[e.ID] = e
	r.byDedup[k] = e
	return e, nil
}
func (r *fakeEventRepo) FindByDedup(ctx context.Context, tid, dk string) (*domain.Event, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeEventRepo) FindByID(ctx context.Context, id string) (*domain.Event, error) {
	return nil, domain.ErrNotFound
}

type fakeSubRepo struct {
	list []domain.Subscription
}
func (r *fakeSubRepo) Create(_ context.Context, s *domain.Subscription) (*domain.Subscription, error) {
	return s, nil
}
func (r *fakeSubRepo) FindByID(_ context.Context, tid, id string) (*domain.Subscription, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) ListByTenant(_ context.Context, tid string) ([]domain.Subscription, error) {
	if tid == "empty" {
		return nil, nil
	}
	if r.list != nil {
		return r.list, nil
	}
	return []domain.Subscription{
		{ID: "sub-1", TenantID: "t1", EventType: "order.created", TargetURL: "https://example.com/webhook"},
		{ID: "sub-2", TenantID: "t1", EventType: "payment.paid", TargetURL: "https://example.com/webhook2"},
	}, nil
}
func (r *fakeSubRepo) Delete(_ context.Context, tid, id string) error { return nil }

type fakeOutboxRepo struct {
	created []*domain.OutboxEntry
}
func (r *fakeOutboxRepo) Create(_ context.Context, e *domain.OutboxEntry) error {
	r.created = append(r.created, e)
	return nil
}
func (r *fakeOutboxRepo) ClaimPending(_ context.Context, now time.Time, limit int) ([]domain.OutboxEntry, error) {
	return nil, nil
}
func (r *fakeOutboxRepo) MarkDelivered(_ context.Context, id string) error { return nil }
func (r *fakeOutboxRepo) MarkFailed(_ context.Context, id string, att int, next time.Time, err string) error {
	return nil
}
func (r *fakeOutboxRepo) MarkDeadLetter(_ context.Context, id, err string) error { return nil }

type fakeClock struct{ t time.Time }
func (c fakeClock) Now() time.Time { return c.t }

type fakeTxManager struct {
	fn func(ctx context.Context, fn func(ctx context.Context) error) error
}
func (m *fakeTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.fn != nil {
		return m.fn(ctx, fn)
	}
	return fn(ctx)
}

// --- Tests ---

func TestIngestEventHappyPath(t *testing.T) {
	t.Parallel()
	outbox := &fakeOutboxRepo{}
	svc := NewIngestService(newFakeEventRepo(), &fakeSubRepo{}, outbox, &fakeTxManager{}, fakeClock{t: time.Now()})
	result, err := svc.IngestEvent(context.Background(), IngestEventRequest{
		TenantID: "t1", EventType: "order.created", Payload: []byte(`{"order":1}`),
		DedupKey: "dk-1", CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsDuplicate {
		t.Fatal("expected non-duplicate")
	}
	if result.EventID != "ev-dk-1" {
		t.Fatalf("got event id %s", result.EventID)
	}
	// Should have created outbox entries only for matching event_type "order.created".
	if len(outbox.created) != 1 {
		t.Fatalf("expected 1 outbox entry, got %d", len(outbox.created))
	}
}

func TestIngestEventIdempotent(t *testing.T) {
	t.Parallel()
	outbox := &fakeOutboxRepo{}
	svc := NewIngestService(newFakeEventRepo(), &fakeSubRepo{}, outbox, &fakeTxManager{}, fakeClock{t: time.Now()})
	// First ingest.
	req := IngestEventRequest{
		TenantID: "t1", EventType: "order.created", Payload: []byte(`{"order":1}`),
		DedupKey: "dk-dup", CreatedBy: "user-1",
	}
	r1, err := svc.IngestEvent(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	// Second ingest with same dedup.
	r2, err := svc.IngestEvent(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.IsDuplicate {
		t.Fatal("expected duplicate")
	}
	if r1.EventID != r2.EventID {
		t.Fatalf("id mismatch: %s vs %s", r1.EventID, r2.EventID)
	}
}

func TestIngestEventNoMatchingSubscriptions(t *testing.T) {
	t.Parallel()
	outbox := &fakeOutboxRepo{}
	subs := &fakeSubRepo{list: []domain.Subscription{
		{ID: "s1", TenantID: "t1", EventType: "other.event", TargetURL: "http://x"},
	}}
	svc := NewIngestService(newFakeEventRepo(), subs, outbox, &fakeTxManager{}, fakeClock{t: time.Now()})
	result, err := svc.IngestEvent(context.Background(), IngestEventRequest{
		TenantID: "t1", EventType: "order.created", Payload: []byte(`{}`),
		DedupKey: "dk-no-match", CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OutboxCreated != 0 {
		t.Fatalf("expected 0 outbox, got %d", result.OutboxCreated)
	}
}

func TestIngestEventValidationError(t *testing.T) {
	t.Parallel()
	svc := NewIngestService(newFakeEventRepo(), &fakeSubRepo{}, &fakeOutboxRepo{}, &fakeTxManager{}, fakeClock{t: time.Now()})
	_, err := svc.IngestEvent(context.Background(), IngestEventRequest{
		TenantID: "", EventType: "", Payload: nil, DedupKey: "",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
