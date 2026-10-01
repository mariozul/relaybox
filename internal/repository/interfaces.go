package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mariozul/relaybox/internal/domain"
)

type EventRepository interface {
	CreateWithOutbox(ctx context.Context, tx pgx.Tx, event *domain.Event, entries []*domain.OutboxEntry) error
	FindByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error)
}

type SubscriptionRepository interface {
	Create(ctx context.Context, tx pgx.Tx, s *domain.Subscription) error
	ListByTenant(ctx context.Context, tenantID string) ([]*domain.Subscription, error)
	Delete(ctx context.Context, tx pgx.Tx, tenantID, id string) error
}

type OutboxRepository interface {
	ClaimPending(ctx context.Context, limit int) ([]*domain.OutboxEntry, error)
	MarkDelivered(ctx context.Context, id string) error
	MarkAttempt(ctx context.Context, id string, nextAt time.Time) error
	DeadLetter(ctx context.Context, id string) error
}
