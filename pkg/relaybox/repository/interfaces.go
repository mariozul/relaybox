// Package repository defines abstract interfaces for data persistence in the
// Relaybox service. These interfaces follow Clean Architecture principles
// (RULE-ARCH-01), ensuring the domain layer depends only on these contracts,
// not on concrete implementations.
package repository

import (
	"context"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// EventRepository defines the contract for persisting and retrieving domain
// events (FR-ING-01, FR-ING-02). Implementations MUST guarantee:
//   - Atomic writes: a Save call that persists an event MUST also enqueue it
//     in the outbox within a single database transaction (RULE-DATA-01).
//   - Idempotency: FindByDedupKey is used to detect duplicate events with the
//     same (tenant_id, dedup_key) pair (FR-ING-03, RULE-DATA-02).
//   - ctx context.Context MUST be propagated to all downstream I/O operations;
//     implementations MUST NOT use context.Background() (RULE-RES-01).
type EventRepository interface {
	// Save persists a new event and returns the saved copy. If the underlying
	// storage enforces deduplication constraints, Save may return
	// domain.ErrDuplicateEvent.
	Save(ctx context.Context, event *domain.Event) (*domain.Event, error)

	// FindByDedupKey retrieves an event by tenant ID and dedup key. Returns
	// domain.ErrNotFound if no matching event exists, or domain.ErrDuplicateEvent
	// if the dedup key already exists for the given tenant.
	FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error)
}

// SubscriptionRepository defines the contract for CRUD operations on webhook
// subscriptions (FR-SUB-01, FR-SUB-02). Implementations MUST enforce strict
// tenant isolation: all queries are scoped by tenantID, and cross-tenant
// access MUST return domain.ErrSubscriptionNotFound (RULE-SEC-01).
type SubscriptionRepository interface {
	// Save creates or updates a subscription. Returns the saved copy.
	Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)

	// FindByID retrieves a subscription by ID, scoped to the given tenant.
	// Returns domain.ErrSubscriptionNotFound if the subscription does not
	// exist or is owned by a different tenant.
	FindByID(ctx context.Context, tenantID string, id string) (*domain.Subscription, error)

	// FindByTenantID returns all subscriptions owned by the specified tenant.
	FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error)

	// Delete removes a subscription by ID, scoped to the given tenant.
	// Returns domain.ErrSubscriptionNotFound if the subscription does not
	// exist or is owned by a different tenant.
	Delete(ctx context.Context, tenantID string, id string) error
}

// OutboxRepository defines the contract for managing outbox entries used in
// the transactional outbox pattern (FR-ING-02, RULE-EVT-02). The outbox table
// serves as a durable, at-least-once delivery queue for domain events.
type OutboxRepository interface {
	// Save persists a new outbox entry. This is called within the same
	// transaction as EventRepository.Save to guarantee atomicity (RULE-DATA-01).
	Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error)

	// FindPending retrieves pending (not yet delivered or dead-lettered) outbox
	// entries, up to the specified limit. Entries with status "pending" or
	// "failed" (eligible for retry) are returned, ordered by next_attempt_at
	// ascending. Returns an empty slice if no pending entries exist.
	FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error)

	// UpdateStatus updates the delivery status of an outbox entry. This is
	// called after a delivery attempt to mark the entry as delivered, failed,
	// or dead-lettered. Returns domain.ErrNotFound if the entry does not exist.
	UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error
}
