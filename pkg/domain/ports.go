package domain

import (
	"context"
	"time"
)

type Clock interface{ Now() time.Time }
type EventStore interface {
	Ingest(ctx context.Context, identity Identity, input EventInput) (Event, error)
}
type SubscriptionStore interface {
	CreateSubscription(ctx context.Context, identity Identity, input SubscriptionInput) (Subscription, error)
	ListSubscriptions(ctx context.Context, identity Identity) ([]Subscription, error)
	DeleteSubscription(ctx context.Context, identity Identity, id string) error
}
type OutboxStore interface {
	Claim(ctx context.Context, limit int, leaseUntil time.Time) ([]Delivery, error)
	Complete(ctx context.Context, delivery Delivery, outcome Outcome) error
}
type Sender interface {
	Send(ctx context.Context, delivery Delivery) (int, error)
}
