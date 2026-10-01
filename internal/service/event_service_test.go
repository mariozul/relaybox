package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/service"
)

// stubEventStore implements domain.EventStore for testing.
type stubEventStore struct {
	mu      sync.Mutex
	events  map[string]domain.Event
	deduped map[string]domain.Event
}

func newStubEventStore() *stubEventStore {
	return &stubEventStore{
		events:  make(map[string]domain.Event),
		deduped: make(map[string]domain.Event),
	}
}

func (s *stubEventStore) CreateEvent(ctx context.Context, tenantID, eventType string, payload []byte, dedupKey string) (domain.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dedupLookup := tenantID + ":" + dedupKey
	if existing, ok := s.deduped[dedupLookup]; ok {
		return existing, nil
	}

	evt := domain.Event{
		ID:        "evt_" + dedupKey,
		TenantID:  tenantID,
		EventType: eventType,
		Payload:   payload,
		DedupKey:  dedupKey,
		CreatedAt: time.Now(),
		CreatedBy: "test",
	}
	s.events[evt.ID] = evt
	s.deduped[dedupLookup] = evt
	return evt, nil
}

func (s *stubEventStore) GetEvent(ctx context.Context, id string) (domain.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	evt, ok := s.events[id]
	if !ok {
		return domain.Event{}, domain.ErrNotFound
	}
	return evt, nil
}

func (s *stubEventStore) GetEventByDedup(ctx context.Context, tenantID, dedupKey string) (domain.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tenantID + ":" + dedupKey
	evt, ok := s.deduped[key]
	if !ok {
		return domain.Event{}, domain.ErrNotFound
	}
	return evt, nil
}

// stubOutboxStore implements domain.OutboxStore for testing.
type stubOutboxStore struct {
	mu     sync.Mutex
	outbox []domain.Outbox
}

func newStubOutboxStore() *stubOutboxStore {
	return &stubOutboxStore{}
}

func (s *stubOutboxStore) CreateOutbox(ctx context.Context, eventID, subscriptionID string) (domain.Outbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ob := domain.Outbox{
		ID:             "ob_" + eventID + "_" + subscriptionID,
		EventID:        eventID,
		SubscriptionID: subscriptionID,
		Status:         domain.OutboxStatusPending,
	}
	s.outbox = append(s.outbox, ob)
	return ob, nil
}

func (s *stubOutboxStore) FetchPending(ctx context.Context, limit int) ([]domain.Outbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pending []domain.Outbox
	for _, ob := range s.outbox {
		if ob.Status == domain.OutboxStatusPending && len(pending) < limit {
			pending = append(pending, ob)
		}
	}
	return pending, nil
}

func (s *stubOutboxStore) MarkDelivered(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, ob := range s.outbox {
		if ob.ID == id {
			s.outbox[i].Status = domain.OutboxStatusDelivered
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *stubOutboxStore) MarkFailed(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, ob := range s.outbox {
		if ob.ID == id {
			s.outbox[i].Status = domain.OutboxStatusFailed
			s.outbox[i].Attempts++
			s.outbox[i].LastError = errMsg
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *stubOutboxStore) MarkDeadLetter(ctx context.Context, id string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, ob := range s.outbox {
		if ob.ID == id {
			s.outbox[i].Status = domain.OutboxStatusDeadLetter
			s.outbox[i].LastError = errMsg
			return nil
		}
	}
	return domain.ErrNotFound
}

// stubSubscriptionStore implements domain.SubscriptionStore for testing.
type stubSubscriptionStore struct {
	mu   sync.Mutex
	subs map[string]domain.Subscription
}

func newStubSubscriptionStore() *stubSubscriptionStore {
	return &stubSubscriptionStore{subs: make(map[string]domain.Subscription)}
}

func (s *stubSubscriptionStore) CreateSubscription(ctx context.Context, tenantID, eventType, targetURL string) (domain.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub := domain.Subscription{
		ID:        "sub_" + eventType + "_" + targetURL[:8],
		TenantID:  tenantID,
		EventType: eventType,
		TargetURL: targetURL,
		CreatedAt: time.Now(),
		CreatedBy: "test",
	}
	s.subs[sub.ID] = sub
	return sub, nil
}

func (s *stubSubscriptionStore) GetSubscription(ctx context.Context, id string) (domain.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subs[id]
	if !ok {
		return domain.Subscription{}, domain.ErrNotFound
	}
	return sub, nil
}

func (s *stubSubscriptionStore) GetSubscriptionsByEventType(ctx context.Context, tenantID, eventType string) ([]domain.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []domain.Subscription
	for _, sub := range s.subs {
		if sub.TenantID == tenantID && sub.EventType == eventType {
			result = append(result, sub)
		}
	}
	return result, nil
}

func (s *stubSubscriptionStore) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []domain.Subscription
	for _, sub := range s.subs {
		if sub.TenantID == tenantID {
			result = append(result, sub)
		}
	}
	return result, nil
}

func (s *stubSubscriptionStore) DeleteSubscription(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.subs, id)
	return nil
}

// stubTxManager implements domain.TxManager for testing.
type stubTxManager struct{}

func (tm *stubTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// --- Actual tests ---

func TestEventService_Ingest_NewEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := service.NewEventService(newStubEventStore(), newStubOutboxStore(), newStubSubscriptionStore(), &stubTxManager{})

	evt, err := svc.Ingest(ctx, "tenant_a", "order.created", []byte(`{}`), "dedup_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.ID == "" {
		t.Fatal("expected non-empty event ID")
	}
	if evt.TenantID != "tenant_a" {
		t.Errorf("expected tenant_a, got %s", evt.TenantID)
	}
}

func TestEventService_Ingest_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newStubEventStore()
	outboxStore := newStubOutboxStore()
	subStore := newStubSubscriptionStore()
	svc := service.NewEventService(store, outboxStore, subStore, &stubTxManager{})

	// First ingest
	evt1, err := svc.Ingest(ctx, "tenant_a", "order.created", []byte(`{}`), "dedup_2")
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	// Second ingest with same dedup key (FR-ING-03, AC-01)
	evt2, err := svc.Ingest(ctx, "tenant_a", "order.created", []byte(`{"changed":true}`), "dedup_2")
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	if evt1.ID != evt2.ID {
		t.Errorf("idempotent ingest returned different ID: %s vs %s", evt1.ID, evt2.ID)
	}
}

func TestEventService_Ingest_CreatesOutbox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newStubEventStore()
	outboxStore := newStubOutboxStore()

	subStore := newStubSubscriptionStore()
	subStore.CreateSubscription(ctx, "tenant_a", "order.created", "https://example.com/webhook")

	svc := service.NewEventService(store, outboxStore, subStore, &stubTxManager{})

	_, err := svc.Ingest(ctx, "tenant_a", "order.created", []byte(`{}`), "dedup_3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pending, err := outboxStore.FetchPending(ctx, 10)
	if err != nil {
		t.Fatalf("fetch pending: %v", err)
	}
	if len(pending) == 0 {
		t.Fatal("expected at least one outbox entry (FR-ING-02, RULE-EVT-02)")
	}
}

func TestEventService_Ingest_NoSubscriptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := service.NewEventService(newStubEventStore(), newStubOutboxStore(), newStubSubscriptionStore(), &stubTxManager{})

	evt, err := svc.Ingest(ctx, "tenant_a", "order.created", []byte(`{}`), "dedup_4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.ID == "" {
		t.Fatal("event should still be created")
	}
}

func TestSubscriptionService_TenantIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	subStore := newStubSubscriptionStore()
	svc := service.NewSubscriptionService(subStore)

	// Create subscription for tenant_a
	sub, err := svc.Create(ctx, "tenant_a", "order.created", "https://a.example.com/webhook")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Tenant b should NOT see tenant a's subscriptions (FR-SUB-02, AC-04)
	listB, err := svc.List(ctx, "tenant_b")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listB) != 0 {
		t.Errorf("tenant_b should not see tenant_a subscriptions, got %d", len(listB))
	}

	// Tenant a CAN see their own
	listA, err := svc.List(ctx, "tenant_a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listA) != 1 {
		t.Errorf("tenant_a should see 1 subscription, got %d", len(listA))
	}
	if listA[0].ID != sub.ID {
		t.Errorf("expected ID %s, got %s", sub.ID, listA[0].ID)
	}
}
