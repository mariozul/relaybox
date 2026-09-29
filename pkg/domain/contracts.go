package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid      = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnavailable  = errors.New("unavailable")
)

type Principal struct{ TenantID, ActorID string }
type Audit struct {
	CreatedAt, UpdatedAt            time.Time
	DeletedAt                       *time.Time
	CreatedBy, UpdatedBy, DeletedBy string
}
type Event struct {
	ID, TenantID, EventType, DedupKey string
	Payload                           []byte
	Audit                             Audit
}
type Subscription struct {
	ID, TenantID, EventType, TargetURL string
	Audit                              Audit
}
type Delivery struct {
	ID, TenantID, EventID, SubscriptionID, TargetURL string
	Payload                                          []byte
	TraceParent, TraceState                          string
	State                                            State
	Attempts                                         int
	LeaseToken                                       string
	LeaseUntil, NextAttemptAt                        time.Time
	Audit                                            Audit
}
type Clock interface{ Now() time.Time }
type EventStore interface {
	Ingest(ctx context.Context, principal Principal, event Event) (Event, error)
}
type SubscriptionStore interface {
	Create(ctx context.Context, principal Principal, subscription Subscription) (Subscription, error)
	List(ctx context.Context, principal Principal, after string, limit int) ([]Subscription, error)
	Delete(ctx context.Context, principal Principal, id string) error
}
type OutboxStore interface {
	Claim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]Delivery, error)
	Complete(ctx context.Context, delivery Delivery, state State, next time.Time) error
}
type Sender interface {
	Send(ctx context.Context, delivery Delivery) (int, error)
}
type Verifier interface {
	Verify(ctx context.Context, requestToken string) (Principal, error)
}
