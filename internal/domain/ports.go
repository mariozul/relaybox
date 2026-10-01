package domain

import (
	"context"
	"time"
)

// EventStore persists and retrieves events (FR-ING-01, FR-ING-02, FR-ING-03).
type EventStore interface {
	CreateEvent(ctx context.Context, tenantID string, eventType string, payload []byte, dedupKey string) (Event, error)
	GetEvent(ctx context.Context, id string) (Event, error)
	GetEventByDedup(ctx context.Context, tenantID string, dedupKey string) (Event, error)
}

// SubscriptionStore manages subscriber mappings (FR-SUB-01, FR-SUB-02).
type SubscriptionStore interface {
	CreateSubscription(ctx context.Context, tenantID string, eventType string, targetURL string) (Subscription, error)
	GetSubscription(ctx context.Context, id string) (Subscription, error)
	GetSubscriptionsByEventType(ctx context.Context, tenantID string, eventType string) ([]Subscription, error)
	ListSubscriptions(ctx context.Context, tenantID string) ([]Subscription, error)
	DeleteSubscription(ctx context.Context, id string) error
}

// OutboxStore manages the transactional outbox for at-least-once delivery (FR-DEL-01, RULE-EVT-02).
type OutboxStore interface {
	CreateOutbox(ctx context.Context, eventID string, subscriptionID string) (Outbox, error)
	FetchPending(ctx context.Context, limit int) ([]Outbox, error)
	MarkDelivered(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error
	MarkDeadLetter(ctx context.Context, id string, errMsg string) error
}

// TxManager abstracts transactional boundaries (RULE-DATA-01).
type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
