package domain

import (
	"context"
	"time"
)

type Store interface {
	Ingest(context.Context, Event) (Event, bool, error)
	CreateSubscription(context.Context, Subscription) (Subscription, error)
	ListSubscriptions(context.Context) ([]Subscription, error)
	DeleteSubscription(context.Context, string) error
	Claim(context.Context, time.Time, int, time.Duration) ([]Delivery, error)
	MarkDelivered(context.Context, Delivery, time.Time) error
	MarkFailed(context.Context, Delivery, time.Time, string, bool) error
	Ping(context.Context) error
}

type Sender interface {
	Send(context.Context, Delivery) (int, error)
}
