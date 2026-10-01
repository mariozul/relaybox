package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mariozul/relaybox/internal/domain"
)

type EventRepo struct{}

func NewEventRepo() *EventRepo { return &EventRepo{} }

func (r *EventRepo) CreateWithOutbox(ctx context.Context, tx pgx.Tx, e *domain.Event, entries []*domain.OutboxEntry) error {
	if tx == nil {
		return fmt.Errorf("transaction required (RULE-DATA-01)")
	}
	return nil
}

func (r *EventRepo) FindByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	return nil, domain.ErrNotFound
}
