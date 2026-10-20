// Package service implements core business logic for the Relaybox multi-tenant
// webhook relay service. It orchestrates domain entities through repository
// interfaces, enforcing strict tenant isolation and business rules
// (RULE-ARCH-02).
package service

import (
	"context"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// SubscriptionService provides CRUD operations for webhook subscriptions,
// enforcing strict multi-tenant isolation (FR-SUB-02, RULE-SEC-01). Every
// operation is scoped by tenantID, ensuring a tenant can only access or modify
// its own subscriptions.
type SubscriptionService interface {
	// CreateSubscription creates a new subscription for the given tenant with
	// the specified event_type and target_url. Returns the created subscription
	// or an error if validation fails.
	CreateSubscription(ctx context.Context, tenantID string, req CreateSubscriptionRequest) (*domain.Subscription, error)

	// GetSubscription retrieves a subscription by ID, scoped to the given tenant.
	// Returns domain.ErrSubscriptionNotFound if the subscription does not exist
	// or is owned by a different tenant (RULE-SEC-01).
	GetSubscription(ctx context.Context, tenantID, id string) (*domain.Subscription, error)

	// ListSubscriptions returns all subscriptions owned by the given tenant.
	ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error)

	// DeleteSubscription removes a subscription by ID, scoped to the given tenant.
	// Returns domain.ErrSubscriptionNotFound if the subscription does not exist
	// or is owned by a different tenant (RULE-SEC-01).
	DeleteSubscription(ctx context.Context, tenantID, id string) error
}

// CreateSubscriptionRequest contains the fields required to create a new
// subscription.
type CreateSubscriptionRequest struct {
	// EventType is the domain event type to subscribe to (e.g., "order.created").
	EventType string `json:"event_type"`
	// TargetURL is the HTTP endpoint that will receive webhook deliveries.
	TargetURL string `json:"target_url"`
}

// subscriptionService is the concrete implementation of SubscriptionService.
// It delegates to a SubscriptionRepository for persistence and enforces
// tenant scoping at the service layer.
type subscriptionService struct {
	repo repository.SubscriptionRepository
}

// NewSubscriptionService creates a new SubscriptionService with the given
// repository implementation.
func NewSubscriptionService(repo repository.SubscriptionRepository) SubscriptionService {
	return &subscriptionService{
		repo: repo,
	}
}

// CreateSubscription validates the request, creates a domain Subscription,
// and persists it. Returns ErrInvalidEventPayload on validation failure.
func (s *subscriptionService) CreateSubscription(ctx context.Context, tenantID string, req CreateSubscriptionRequest) (*domain.Subscription, error) {
	if tenantID == "" {
		return nil, domain.ErrInvalidEventPayload
	}
	if req.EventType == "" {
		return nil, domain.ErrInvalidEventPayload
	}
	if req.TargetURL == "" {
		return nil, domain.ErrInvalidEventPayload
	}

	sub, err := domain.NewSubscription(tenantID, req.EventType, req.TargetURL, "admin", time.Now().UTC())
	if err != nil {
		return nil, err
	}

	saved, err := s.repo.Save(ctx, sub)
	if err != nil {
		return nil, err
	}

	return saved, nil
}

// GetSubscription retrieves a subscription by ID, scoped to the tenant.
// The repository enforces that only the owning tenant can access the
// subscription (RULE-SEC-01).
func (s *subscriptionService) GetSubscription(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	if tenantID == "" {
		return nil, domain.ErrInvalidEventPayload
	}
	if id == "" {
		return nil, domain.ErrSubscriptionNotFound
	}

	sub, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

// ListSubscriptions returns all subscriptions owned by the given tenant.
func (s *subscriptionService) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	if tenantID == "" {
		return nil, domain.ErrInvalidEventPayload
	}

	subs, err := s.repo.FindByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return subs, nil
}

// DeleteSubscription removes a subscription by ID, scoped to the tenant.
// The repository enforces that only the owning tenant can delete the
// subscription (RULE-SEC-01).
func (s *subscriptionService) DeleteSubscription(ctx context.Context, tenantID, id string) error {
	if tenantID == "" {
		return domain.ErrInvalidEventPayload
	}
	if id == "" {
		return domain.ErrSubscriptionNotFound
	}

	err := s.repo.Delete(ctx, tenantID, id)
	if err != nil {
		return err
	}

	return nil
}
