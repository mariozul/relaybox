package service

import (
	"context"

	"github.com/mariozul/relaybox/internal/domain"
)

// SubscriptionService manages webhook subscriptions for tenants.
type SubscriptionService struct {
	store domain.SubscriptionStore
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(store domain.SubscriptionStore) *SubscriptionService {
	return &SubscriptionService{store: store}
}

// Create adds a new subscription.
func (s *SubscriptionService) Create(ctx context.Context, tenantID, eventType, targetURL string) (domain.Subscription, error) {
	return s.store.CreateSubscription(ctx, tenantID, eventType, targetURL)
}

// Get retrieves a subscription by ID.
func (s *SubscriptionService) Get(ctx context.Context, id string) (domain.Subscription, error) {
	return s.store.GetSubscription(ctx, id)
}

// List returns all subscriptions for a tenant (tenant-scoped per RULE-SEC-01).
func (s *SubscriptionService) List(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return s.store.ListSubscriptions(ctx, tenantID)
}

// GetByEventType returns subscriptions matching tenant + event_type.
func (s *SubscriptionService) GetByEventType(ctx context.Context, tenantID, eventType string) ([]domain.Subscription, error) {
	return s.store.GetSubscriptionsByEventType(ctx, tenantID, eventType)
}

// Delete removes a subscription (soft-delete per RULE-DATA-03).
func (s *SubscriptionService) Delete(ctx context.Context, id string) error {
	return s.store.DeleteSubscription(ctx, id)
}
