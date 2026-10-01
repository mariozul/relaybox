package service

import (
	"context"
	"sync"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

// mockClock returns a fixed time for deterministic tests.
type mockClock struct {
	mu sync.RWMutex
	t  time.Time
}

func (c *mockClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.t
}

func (c *mockClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func newMockClock() *mockClock {
	return &mockClock{t: time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)}
}

// mockEventRepo simulates the event repository for unit tests.
type mockEventRepo struct {
	mu     sync.RWMutex
	events map[string]*domain.Event
}

func newMockEventRepo() *mockEventRepo {
	return &mockEventRepo{events: make(map[string]*domain.Event)}
}

func (r *mockEventRepo) CreateOrGet(ctx context.Context, event domain.Event) (*domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := event.TenantID + ":" + event.DedupKey
	if existing, ok := r.events[key]; ok {
		return existing, nil
	}
	event.ID = key // deterministic ID for tests
	r.events[key] = &event
	return &event, nil
}

// mockSubscriptionRepo simulates the subscription repository.
type mockSubscriptionRepo struct {
	mu   sync.RWMutex
	subs map[string]*domain.Subscription
}

func newMockSubscriptionRepo() *mockSubscriptionRepo {
	return &mockSubscriptionRepo{subs: make(map[string]*domain.Subscription)}
}

func (r *mockSubscriptionRepo) Create(ctx context.Context, sub domain.Subscription) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := sub.TenantID + ":" + sub.EventType + ":" + sub.TargetURL
	if _, ok := r.subs[id]; ok {
		return nil, domain.ErrConflict
	}
	sub.ID = id
	r.subs[id] = &sub
	return &sub, nil
}

func (r *mockSubscriptionRepo) GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sub, ok := r.subs[id]
	if !ok || sub.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return sub, nil
}

func (r *mockSubscriptionRepo) ListByTenant(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.Subscription
	for _, s := range r.subs {
		if s.TenantID == tenantID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (r *mockSubscriptionRepo) ListByEventType(ctx context.Context, tenantID, eventType string) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.Subscription
	for _, s := range r.subs {
		if s.TenantID == tenantID && s.EventType == eventType {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (r *mockSubscriptionRepo) Delete(ctx context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub, ok := r.subs[id]
	if !ok || sub.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.subs, id)
	return nil
}

// mockOutboxRepo simulates the outbox repository.
type mockOutboxRepo struct {
	mu       sync.RWMutex
	messages []domain.OutboxMessage
}

func newMockOutboxRepo() *mockOutboxRepo {
	return &mockOutboxRepo{}
}

func (r *mockOutboxRepo) CreateBatch(ctx context.Context, msgs []domain.OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range msgs {
		msg := m
		msg.ID = "ob-" + m.EventID + "-" + m.SubscriptionID
		r.messages = append(r.messages, msg)
	}
	return nil
}

func (r *mockOutboxRepo) ClaimPending(ctx context.Context, limit int) ([]domain.OutboxMessage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.OutboxMessage
	for _, m := range r.messages {
		if m.Status == domain.OutboxStatusPending {
			result = append(result, m)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *mockOutboxRepo) MarkDelivered(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.messages {
		if r.messages[i].ID == id {
			r.messages[i].Status = domain.OutboxStatusDelivered
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *mockOutboxRepo) MarkRetry(ctx context.Context, id string, nextAttemptAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.messages {
		if r.messages[i].ID == id {
			r.messages[i].Attempts++
			r.messages[i].NextAttemptAt = nextAttemptAt
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *mockOutboxRepo) DeadLetter(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.messages {
		if r.messages[i].ID == id {
			r.messages[i].Status = domain.OutboxStatusDeadLetter
			return nil
		}
	}
	return domain.ErrNotFound
}
