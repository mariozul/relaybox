package service

import (
	"context"
	"fmt"

	"github.com/mariozul/relaybox/internal/domain"
)

// EventService handles event ingestion with idempotency and transactional outbox.
type EventService struct {
	eventRepo  domain.EventRepository
	outboxRepo domain.OutboxRepository
	subRepo    domain.SubscriptionRepository
	clock      domain.Clock
}

// NewEventService creates an EventService with required dependencies.
func NewEventService(
	eventRepo domain.EventRepository,
	outboxRepo domain.OutboxRepository,
	subRepo domain.SubscriptionRepository,
	clock domain.Clock,
) *EventService {
	return &EventService{
		eventRepo:  eventRepo,
		outboxRepo: outboxRepo,
		subRepo:    subRepo,
		clock:      clock,
	}
}

// IngestInput is the payload for the Ingest operation.
type IngestInput struct {
	TenantID  string
	EventType string
	Payload   []byte
	DedupKey  string
	CreatedBy string
}

// IngestResult holds the result of an ingest operation.
type IngestResult struct {
	EventID        string
	OutboxRowCount int
}

// Ingest persists an event and enqueues outbox rows in a single logical flow.
// Idempotency is enforced by the unique (tenant_id, dedup_key) constraint.
func (s *EventService) Ingest(ctx context.Context, input IngestInput) (*IngestResult, error) {
	event := domain.NewEvent(input.TenantID, input.EventType, input.Payload, input.DedupKey, input.CreatedBy, s.clock.Now())

	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("event validation: %w", err)
	}

	// Find matching subscriptions for this tenant+event_type.
	subs, err := s.subRepo.ListByEventType(ctx, input.TenantID, input.EventType)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}

	// Create outbox messages for each matching subscription.
	outboxMessages := make([]domain.OutboxMessage, 0, len(subs))
	now := s.clock.Now()
	for _, sub := range subs {
		deliveryID := fmt.Sprintf("%s:%s", event.DedupKey, sub.ID)
		msg := domain.NewOutboxMessage("", sub.ID, deliveryID, input.CreatedBy, now)
		outboxMessages = append(outboxMessages, msg)
	}

	// Persist event (idempotent via unique constraint).
	savedEvent, err := s.eventRepo.CreateOrGet(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("create or get event: %w", err)
	}

	// Fill in the resolved event ID for all outbox messages.
	for i := range outboxMessages {
		outboxMessages[i].EventID = savedEvent.ID
	}

	// Enqueue outbox rows.
	if len(outboxMessages) > 0 {
		if err := s.outboxRepo.CreateBatch(ctx, outboxMessages); err != nil {
			return nil, fmt.Errorf("create outbox batch: %w", err)
		}
	}

	return &IngestResult{
		EventID:        savedEvent.ID,
		OutboxRowCount: len(outboxMessages),
	}, nil
}
