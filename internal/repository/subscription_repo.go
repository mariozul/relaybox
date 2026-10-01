package repository

import (
	"context"

	"github.com/mariozul/relaybox/internal/domain"
)

// SubscriptionRepository defines persistence operations for domain.Subscription entities.
type SubscriptionRepository interface {
	// Create inserts a new subscription scoped to the subscription's tenant.
	Create(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)

	// FindByID returns the subscription by ID, scoped to the given tenant.
	// Returns domain.ErrNotFound if the subscription doesn't exist or belongs
	// to a different tenant.
	FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error)

	// ListByTenant returns all subscriptions belonging to the given tenant.
	ListByTenant(ctx context.Context, tenantID string) ([]domain.Subscription, error)

	// Delete removes the subscription by ID, scoped to the given tenant.
	// Returns domain.ErrNotFound if the subscription doesn't exist or belongs
	// to a different tenant.
	Delete(ctx context.Context, tenantID, id string) error
}
