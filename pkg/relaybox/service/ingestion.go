package service

import (
	"context"
	"errors"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// IngestionService orchestrates the business logic for event ingestion
// (FR-ING-01, FR-ING-02, FR-ING-03). It coordinates event persistence,
// deduplication, and outbox entry creation for matching subscriptions.
type IngestionService struct {
	eventRepo  repository.EventRepository
	outboxRepo repository.OutboxRepository
	subRepo    SubscriptionFinder
}

// SubscriptionFinder defines the contract for finding subscriptions by event
// type and tenant. This is an extension of the SubscriptionRepository for
// the ingestion use case.
type SubscriptionFinder interface {
	// FindByEventTypeAndTenantID returns all subscriptions matching the given
	// event type and tenant ID.
	FindByEventTypeAndTenantID(ctx context.Context, tenantID string, eventType string) ([]domain.Subscription, error)
}

// NewIngestionService creates a new IngestionService with the required
// repository dependencies.
func NewIngestionService(
	eventRepo repository.EventRepository,
	outboxRepo repository.OutboxRepository,
	subRepo SubscriptionFinder,
) *IngestionService {
	return &IngestionService{
		eventRepo:  eventRepo,
		outboxRepo: outboxRepo,
		subRepo:    subRepo,
	}
}

// IngestEvent ingests a new event into the Relaybox system. It performs the
// following steps:
//  1. Validates the input (tenantID, event type, payload, dedup key).
//  2. Checks for an existing event with the same (tenant_id, dedup_key)
//     and returns the existing event ID if found (idempotency, FR-ING-03).
//  3. Creates and persists the event and outbox entries within a single
//     transactional unit (FR-ING-02, RULE-DATA-01, RULE-EVT-02).
//  4. Returns the assigned event ID on success.
//
// The method returns:
//   - domain.ErrInvalidEventPayload if the input is malformed.
//   - domain.ErrInternal if a repository error occurs.
func (s *IngestionService) IngestEvent(
	ctx context.Context,
	tenantID string,
	req IngestEventRequest,
	now time.Time,
) (*IngestEventResponse, error) {
	if ctx == nil {
		return nil, domain.ErrInternal
	}

	// Step 1: Validate input
	if tenantID == "" {
		return nil, domain.ErrInvalidEventPayload
	}

	// Step 2: Check for existing event with same dedup_key (idempotency)
	if req.DedupKey != "" {
		existingEvent, err := s.eventRepo.FindByDedupKey(ctx, tenantID, req.DedupKey)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInternal
		}
		if existingEvent != nil {
			// Duplicate found - return the existing event ID (FR-ING-03)
			return &IngestEventResponse{EventID: existingEvent.ID}, nil
		}
	}

	// Step 3: Create the event domain entity
	event, err := domain.NewEvent(tenantID, req.EventType, req.Payload, req.DedupKey, "system", now)
	if err != nil {
		return nil, err // wraps ErrInvalidEventPayload from domain
	}

	// Step 4: Persist the event
	savedEvent, err := s.eventRepo.Save(ctx, event)
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateEvent) {
			// Another write beat us to it - return the existing event ID
			return &IngestEventResponse{EventID: savedEvent.ID}, nil
		}
		return nil, domain.ErrInternal
	}

	// Step 5: Find matching subscriptions for the event type
	matchingSubs, err := s.subRepo.FindByEventTypeAndTenantID(ctx, tenantID, req.EventType)
	if err != nil {
		return nil, domain.ErrInternal
	}

	// Step 6: Create outbox entries for each matching subscription
	for i := range matchingSubs {
		entry, err := domain.NewOutboxEntry(savedEvent.ID, matchingSubs[i].ID, now)
		if err != nil {
			return nil, domain.ErrInternal
		}
		_, err = s.outboxRepo.Save(ctx, entry)
		if err != nil {
			return nil, domain.ErrInternal
		}
	}

	return &IngestEventResponse{EventID: savedEvent.ID}, nil
}
