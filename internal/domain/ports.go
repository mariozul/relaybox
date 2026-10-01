package domain

import (
	"context"
	"time"
)

// Clock abstracts time operations for testability.
type Clock interface {
	Now() time.Time
}

// EventRepository persists and retrieves events.
type EventRepository interface {
	// CreateOrGet inserts an event or returns the existing one on dedup-key conflict.
	// Must be scoped per tenant (RULE-SEC-01).
	CreateOrGet(ctx context.Context, event Event) (*Event, error)
}

// SubscriptionRepository manages tenant-scoped subscriptions.
type SubscriptionRepository interface {
	Create(ctx context.Context, sub Subscription) (*Subscription, error)
	GetByID(ctx context.Context, tenantID, id string) (*Subscription, error)
	ListByTenant(ctx context.Context, tenantID string) ([]Subscription, error)
	ListByEventType(ctx context.Context, tenantID, eventType string) ([]Subscription, error)
	Delete(ctx context.Context, tenantID, id string) error
}

// OutboxRepository manages the transactional outbox.
type OutboxRepository interface {
	// CreateBatch inserts outbox rows within a transaction context.
	CreateBatch(ctx context.Context, messages []OutboxMessage) error
	// ClaimPending locks and returns up to limit pending rows ordered by next_attempt_at.
	ClaimPending(ctx context.Context, limit int) ([]OutboxMessage, error)
	// MarkDelivered marks an outbox row as successfully delivered.
	MarkDelivered(ctx context.Context, id string) error
	// MarkRetry increments attempts and sets next_attempt_at using backoff.
	MarkRetry(ctx context.Context, id string, nextAttemptAt time.Time) error
	// DeadLetter marks an outbox row as permanently failed.
	DeadLetter(ctx context.Context, id string) error
}
