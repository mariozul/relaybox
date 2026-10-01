package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/repository"
)

type SubscriptionService struct {
	repo  repository.SubscriptionRepository
	clock domain.Clock
}

func NewSubscriptionService(repo repository.SubscriptionRepository, clock domain.Clock) *SubscriptionService {
	return &SubscriptionService{repo: repo, clock: clock}
}

func (s *SubscriptionService) Create(ctx context.Context, tenantID, eventType, targetURL, createdBy string) (*domain.Subscription, error) {
	if tenantID == "" {
		return nil, domain.ErrTenantMissing
	}
	if eventType == "" || targetURL == "" {
		return nil, domain.ErrInvalidEventPayload
	}
	now := s.clock.Now()
	sub := &domain.Subscription{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		EventType: eventType,
		TargetURL: targetURL,
		CreatedAt: now,
		CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, nil, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *SubscriptionService) List(ctx context.Context, tenantID string) ([]*domain.Subscription, error) {
	if tenantID == "" {
		return nil, domain.ErrTenantMissing
	}
	return s.repo.ListByTenant(ctx, tenantID)
}

func (s *SubscriptionService) Delete(ctx context.Context, tenantID, id string) error {
	if tenantID == "" {
		return domain.ErrTenantMissing
	}
	err := s.repo.Delete(ctx, nil, tenantID, id)
	if errors.Is(err, domain.ErrSubscriptionNotFound) {
		return domain.ErrSubscriptionNotFound
	}
	return err
}
