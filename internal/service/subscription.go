package service

import (
	"context"
	"fmt"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/repository"
)

// SubscriptionService manages tenant-scoped subscription CRUD.
type SubscriptionService struct {
	repo repository.SubscriptionRepository
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(repo repository.SubscriptionRepository) *SubscriptionService {
	return &SubscriptionService{repo: repo}
}

// CreateSubscriptionRequest is the input for creating a subscription.
type CreateSubscriptionRequest struct {
	TenantID  string
	EventType string
	TargetURL string
	CreatedBy string
}

// CreateSubscription creates a new subscription for the tenant.
func (s *SubscriptionService) CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*domain.Subscription, error) {
	sub := &domain.Subscription{
		TenantID:  req.TenantID,
		EventType: req.EventType,
		TargetURL: req.TargetURL,
		CreatedBy: req.CreatedBy,
	}
	if sub.CreatedBy == "" {
		sub.CreatedBy = "system"
	}
	if err := sub.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, sub)
	if err != nil {
		return nil, fmt.Errorf("subscription: create: %w", err)
	}
	return created, nil
}

// GetSubscription returns a subscription by ID, scoped to tenant.
func (s *SubscriptionService) GetSubscription(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	sub, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("subscription: get: %w", err)
	}
	return sub, nil
}

// ListSubscriptions returns all subscriptions for a tenant.
func (s *SubscriptionService) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	subs, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("subscription: list: %w", err)
	}
	return subs, nil
}

// DeleteSubscription removes a subscription by ID, scoped to tenant.
func (s *SubscriptionService) DeleteSubscription(ctx context.Context, tenantID, id string) error {
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return fmt.Errorf("subscription: delete: %w", err)
	}
	return nil
}
