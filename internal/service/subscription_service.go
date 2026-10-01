package service

import (
	"context"
	"fmt"

	"github.com/mariozul/relaybox/internal/domain"
)

// SubscriptionService handles CRUD for tenant-scoped subscriptions.
type SubscriptionService struct {
	subRepo domain.SubscriptionRepository
	clock   domain.Clock
}

// NewSubscriptionService creates a SubscriptionService.
func NewSubscriptionService(subRepo domain.SubscriptionRepository, clock domain.Clock) *SubscriptionService {
	return &SubscriptionService{subRepo: subRepo, clock: clock}
}

// CreateInput holds data to create a subscription.
type CreateInput struct {
	TenantID  string
	EventType string
	TargetURL string
	CreatedBy string
}

// Create adds a subscription for the authenticated tenant.
func (s *SubscriptionService) Create(ctx context.Context, input CreateInput) (*domain.Subscription, error) {
	sub := domain.NewSubscription(input.TenantID, input.EventType, input.TargetURL, input.CreatedBy, s.clock.Now())
	if err := sub.Validate(); err != nil {
		return nil, fmt.Errorf("subscription validation: %w", err)
	}
	created, err := s.subRepo.Create(ctx, sub)
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}
	return created, nil
}

// GetByID returns a subscription by ID, scoped to the tenant.
func (s *SubscriptionService) GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	sub, err := s.subRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return sub, nil
}

// List returns all subscriptions for a tenant.
func (s *SubscriptionService) List(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	subs, err := s.subRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	return subs, nil
}

// Delete removes a subscription, scoped to the tenant.
func (s *SubscriptionService) Delete(ctx context.Context, tenantID, id string) error {
	if err := s.subRepo.Delete(ctx, tenantID, id); err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}
