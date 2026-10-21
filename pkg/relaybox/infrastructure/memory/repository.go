// Package memory provides in-memory implementations of repository interfaces
// for development, testing, and demonstration purposes.
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// ---------------------------------------------------------------------------
// Compile-time interface satisfaction checks
// ---------------------------------------------------------------------------

var (
	_ repository.EventRepository      = (*InMemoryEventRepo)(nil)
	_ repository.SubscriptionRepository = (*InMemorySubscriptionRepo)(nil)
	_ repository.OutboxRepository     = (*InMemoryOutboxRepo)(nil)
	_ repository.TransactionManager   = (*InMemoryTxManager)(nil)
)

// ---------------------------------------------------------------------------
// InMemoryEventRepo
// ---------------------------------------------------------------------------

// InMemoryEventRepo is an in-memory implementation of EventRepository.
// Uses a map keyed by (tenantID, dedupKey) for idempotency checks.
type InMemoryEventRepo struct {
	mu     sync.RWMutex
	events []domain.Event
}

// NewInMemoryEventRepo creates a new empty in-memory event repository.
func NewInMemoryEventRepo() *InMemoryEventRepo {
	return &InMemoryEventRepo{
		events: make([]domain.Event, 0),
	}
}

// CreateEvent inserts a new event. Returns domain.ErrEventAlreadyExists if
// (tenant_id, dedup_key) already exists.
func (r *InMemoryEventRepo) CreateEvent(ctx context.Context, event *domain.Event) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate (tenant_id, dedup_key)
	for _, e := range r.events {
		if e.TenantID == event.TenantID && e.DedupKey == event.DedupKey {
			return e.ID, fmt.Errorf("%w: duplicate (tenant_id, dedup_key)", domain.ErrEventAlreadyExists)
		}
	}

	// Generate ID if not set
	id := event.ID
	if id == "" {
		id = uuid.New().String()
	}

	entry := *event
	entry.ID = id
	r.events = append(r.events, entry)
	return id, nil
}

// GetEventByDedupKey retrieves an event by tenant and dedup key.
func (r *InMemoryEventRepo) GetEventByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, e := range r.events {
		if e.TenantID == tenantID && e.DedupKey == dedupKey {
			return &e, nil
		}
	}
	return nil, domain.ErrSubscriptionNotFound
}

// ---------------------------------------------------------------------------
// InMemorySubscriptionRepo
// ---------------------------------------------------------------------------

// InMemorySubscriptionRepo is an in-memory implementation of SubscriptionRepository.
type InMemorySubscriptionRepo struct {
	mu            sync.RWMutex
	subscriptions []domain.Subscription
	counter       int // for generating sequential IDs
}

// NewInMemorySubscriptionRepo creates a new empty in-memory subscription repository.
func NewInMemorySubscriptionRepo() *InMemorySubscriptionRepo {
	return &InMemorySubscriptionRepo{
		subscriptions: make([]domain.Subscription, 0),
	}
}

// CreateSubscription inserts a new subscription and returns its ID.
func (r *InMemorySubscriptionRepo) CreateSubscription(ctx context.Context, sub *domain.Subscription) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	id := fmt.Sprintf("sub-%d", r.counter)
	entry := *sub
	entry.ID = id
	r.subscriptions = append(r.subscriptions, entry)
	return id, nil
}

// GetSubscriptionByID retrieves a subscription scoped to the requesting tenant.
func (r *InMemorySubscriptionRepo) GetSubscriptionByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, s := range r.subscriptions {
		if s.ID == id {
			if s.TenantID != tenantID {
				return nil, nil // tenant does not own this subscription
			}
			return &s, nil
		}
	}
	return nil, nil
}

// ListSubscriptions retrieves all subscriptions for a tenant.
func (r *InMemorySubscriptionRepo) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.Subscription
	for _, s := range r.subscriptions {
		if s.TenantID == tenantID {
			result = append(result, s)
		}
	}
	return result, nil
}

// UpdateSubscription modifies an existing subscription.
func (r *InMemorySubscriptionRepo) UpdateSubscription(ctx context.Context, sub *domain.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, s := range r.subscriptions {
		if s.ID == sub.ID && s.TenantID == sub.TenantID {
			r.subscriptions[i] = *sub
			return nil
		}
	}
	return domain.ErrSubscriptionNotFound
}

// DeleteSubscription removes a subscription.
func (r *InMemorySubscriptionRepo) DeleteSubscription(ctx context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, s := range r.subscriptions {
		if s.ID == id && s.TenantID == tenantID {
			r.subscriptions = append(r.subscriptions[:i], r.subscriptions[i+1:]...)
			return nil
		}
	}
	return domain.ErrSubscriptionNotFound
}

// ---------------------------------------------------------------------------
// InMemoryOutboxRepo
// ---------------------------------------------------------------------------

// InMemoryOutboxRepo is an in-memory implementation of OutboxRepository.
type InMemoryOutboxRepo struct {
	mu      sync.RWMutex
	entries []domain.OutboxEntry
}

// NewInMemoryOutboxRepo creates a new empty in-memory outbox repository.
func NewInMemoryOutboxRepo() *InMemoryOutboxRepo {
	return &InMemoryOutboxRepo{
		entries: make([]domain.OutboxEntry, 0),
	}
}

// EnqueueEvent adds a new entry to the outbox.
func (r *InMemoryOutboxRepo) EnqueueEvent(ctx context.Context, entry *domain.OutboxEntry) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := entry.ID
	if id == "" {
		id = uuid.New().String()
	}
	e := *entry
	e.ID = id
	r.entries = append(r.entries, e)
	return id, nil
}

// GetPendingOutboxEntries retrieves entries ready for delivery.
func (r *InMemoryOutboxRepo) GetPendingOutboxEntries(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.OutboxEntry
	now := time.Now()
	for _, e := range r.entries {
		if e.Status == "pending" && (e.NextAttemptAt.IsZero() || !e.NextAttemptAt.After(now)) {
			result = append(result, e)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// MarkOutboxEntryProcessed updates the status and attempt info of an outbox entry.
func (r *InMemoryOutboxRepo) MarkOutboxEntryProcessed(ctx context.Context, id string, status string, attempts int, nextAttemptAt *time.Time, lastAttemptError *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, e := range r.entries {
		if e.ID == id {
			r.entries[i].Status = status
			r.entries[i].Attempts = attempts
			r.entries[i].LastAttemptAt = time.Now()
			if nextAttemptAt != nil {
				r.entries[i].NextAttemptAt = *nextAttemptAt
			}
			if lastAttemptError != nil {
				r.entries[i].LastAttemptError = *lastAttemptError
			}
			return nil
		}
	}
	return fmt.Errorf("outbox entry %s not found", id)
}

// ---------------------------------------------------------------------------
// InMemoryTxManager
// ---------------------------------------------------------------------------

// InMemoryTxManager is a no-op transaction manager for in-memory operations.
// Since in-memory operations are inherently atomic, transactions are simulated.
type InMemoryTxManager struct{}

// NewInMemoryTxManager creates a new InMemoryTxManager.
func NewInMemoryTxManager() *InMemoryTxManager {
	return &InMemoryTxManager{}
}

// BeginTx starts a new "transaction". Returns a sentinel value.
func (m *InMemoryTxManager) BeginTx(ctx context.Context) (interface{}, error) {
	return &inMemoryTx{committed: false}, nil
}

// CommitTx marks the transaction as committed.
func (m *InMemoryTxManager) CommitTx(ctx context.Context, tx interface{}) error {
	if t, ok := tx.(*inMemoryTx); ok {
		t.committed = true
		return nil
	}
	return fmt.Errorf("invalid transaction type")
}

// RollbackTx marks the transaction as rolled back.
func (m *InMemoryTxManager) RollbackTx(ctx context.Context, tx interface{}) error {
	if t, ok := tx.(*inMemoryTx); ok {
		t.committed = false
		return nil
	}
	return fmt.Errorf("invalid transaction type")
}

// ExecInTx executes a function within a simulated transaction with auto rollback on error.
func (m *InMemoryTxManager) ExecInTx(ctx context.Context, fn func(txCtx context.Context, tx interface{}) error) error {
	tx, err := m.BeginTx(ctx)
	if err != nil {
		return err
	}

	if err := fn(ctx, tx); err != nil {
		_ = m.RollbackTx(ctx, tx)
		return err
	}

	return m.CommitTx(ctx, tx)
}

type inMemoryTx struct {
	committed bool
}
