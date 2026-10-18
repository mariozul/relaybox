package repository

import (
	"context"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// SubscriptionRepository defines the contract for webhook subscriber persistence.
type SubscriptionRepository interface {
	// CreateSubscription inserts a new subscription and returns its ID.
	CreateSubscription(ctx context.Context, sub *domain.Subscription) (string, error)

	// GetSubscriptionByID retrieves a subscription scoped to the requesting tenant.
	GetSubscriptionByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error)

	// ListSubscriptions retrieves all subscriptions for a tenant.
	ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error)

	// UpdateSubscription modifies an existing subscription.
	UpdateSubscription(ctx context.Context, sub *domain.Subscription) error

	// DeleteSubscription removes a subscription.
	DeleteSubscription(ctx context.Context, tenantID, id string) error
}
