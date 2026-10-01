package service

import (
	"context"
	"fmt"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/repository"
)

const MaxEventPayloadBytes = 256 * 1024

type IngestService struct {
	eventRepo        repository.EventRepository
	subscriptionRepo repository.SubscriptionRepository
	outboxRepo       repository.OutboxRepository
	txManager        repository.TxManager
	clock            domain.Clock
}

func NewIngestService(
	er repository.EventRepository, sr repository.SubscriptionRepository,
	or repository.OutboxRepository, tm repository.TxManager, c domain.Clock,
) *IngestService {
	return &IngestService{eventRepo: er, subscriptionRepo: sr, outboxRepo: or, txManager: tm, clock: c}
}

type IngestEventRequest struct {
	TenantID, EventType string
	Payload               []byte
	DedupKey, CreatedBy string
}

type IngestEventResult struct {
	EventID       string
	IsDuplicate   bool
	OutboxCreated int
}

func (s *IngestService) IngestEvent(ctx context.Context, req IngestEventRequest) (*IngestEventResult, error) {
	now := s.clock.Now()
	event := &domain.Event{
		TenantID:  req.TenantID,
		EventType: req.EventType,
		Payload:   req.Payload,
		DedupKey:  req.DedupKey,
		Status:    domain.EventStatusPending,
		CreatedAt: now,
		CreatedBy: req.CreatedBy,
	}
	if event.CreatedBy == "" {
		event.CreatedBy = "system"
	}
	if err := event.Validate(MaxEventPayloadBytes); err != nil {
		return nil, err
	}

	var result *IngestEventResult
	err := s.txManager.WithTx(ctx, func(ctx context.Context) error {
		created, err := s.eventRepo.Create(ctx, event)
		if err == domain.ErrConflict {
			result = &IngestEventResult{EventID: created.ID, IsDuplicate: true}
			return nil
		}
		if err != nil {
			return fmt.Errorf("ingest: create event: %w", err)
		}
		event.ID = created.ID

		subs, err := s.subscriptionRepo.ListByTenant(ctx, req.TenantID)
		if err != nil {
			return fmt.Errorf("ingest: list subscriptions: %w", err)
		}

		outboxCount := 0
		for _, sub := range subs {
			if sub.EventType != req.EventType {
				continue
			}
			entry := &domain.OutboxEntry{
				EventID:        event.ID,
				SubscriptionID: sub.ID,
				Status:         domain.OutboxStatusPending,
				NextAttemptAt:  now,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := s.outboxRepo.Create(ctx, entry); err != nil {
				return fmt.Errorf("ingest: create outbox entry: %w", err)
			}
			outboxCount++
		}
		result = &IngestEventResult{
			EventID:       event.ID,
			IsDuplicate:   false,
			OutboxCreated: outboxCount,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
