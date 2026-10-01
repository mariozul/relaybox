package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mariozul/relaybox/internal/domain"
)

type SubscriptionRepo struct{}

func NewSubscriptionRepo() *SubscriptionRepo { return &SubscriptionRepo{} }

func (r *SubscriptionRepo) Create(ctx context.Context, tx pgx.Tx, s *domain.Subscription) error {
	if tx == nil {
		return fmt.Errorf("transaction required")
	}
	return nil
}

func (r *SubscriptionRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Subscription, error) {
	return nil, nil
}

func (r *SubscriptionRepo) Delete(ctx context.Context, tx pgx.Tx, tenantID, id string) error {
	return nil
}
