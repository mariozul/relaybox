package postgres

import (
	"context"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

type OutboxRepo struct{}

func NewOutboxRepo() *OutboxRepo { return &OutboxRepo{} }

func (r *OutboxRepo) ClaimPending(ctx context.Context, limit int) ([]*domain.OutboxEntry, error) {
	return nil, nil
}

func (r *OutboxRepo) MarkDelivered(ctx context.Context, id string) error {
	return nil
}

func (r *OutboxRepo) MarkAttempt(ctx context.Context, id string, nextAt time.Time) error {
	return nil
}

func (r *OutboxRepo) DeadLetter(ctx context.Context, id string) error {
	return nil
}
