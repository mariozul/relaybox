// Package service implements application service layer orchestrating domain
// operations across repository boundaries (Clean Architecture service layer).
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/dto"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// EventIngestionService defines the contract for ingesting events into the system.
// It orchestrates event persistence and outbox enqueuing in a single atomic
// transaction (FR-ING-01, FR-ING-02) with idempotent dedup-key handling
// (FR-ING-03).
type EventIngestionService interface {
	// IngestEvent persists an event and enqueues it in the outbox atomically.
	// Returns the event ID on success.
	// Returns domain.ErrInvalidInput if request or tenantID is empty.
	// Returns domain.ErrEventAlreadyExists (with existing ID) if (tenant, dedup_key)
	// already exists — this is the idempotent path (FR-ING-03).
	IngestEvent(ctx context.Context, tenantID string, req *dto.IngestEventRequest) (string, error)
}

type eventIngestionService struct {
	eventRepo  repository.EventRepository
	outboxRepo repository.OutboxRepository
	txManager  repository.TransactionManager
	clock      func() time.Time
}

// NewEventIngestionService creates a new EventIngestionService.
// The clock parameter enables deterministic time injection for testing.
func NewEventIngestionService(
	eventRepo repository.EventRepository,
	outboxRepo repository.OutboxRepository,
	txManager repository.TransactionManager,
	clock func() time.Time,
) EventIngestionService {
	return &eventIngestionService{
		eventRepo:  eventRepo,
		outboxRepo: outboxRepo,
		txManager:  txManager,
		clock:      clock,
	}
}

// IngestEvent implements the EventIngestionService interface.
//
// Transactional flow (RULE-DATA-01):
//  1. Validates inputs (FR-ING-01: tenant from context, never body — RULE-SEC-01).
//  2. Begins a database transaction.
//  3. Creates the event. If ErrEventAlreadyExists, returns existing ID (FR-ING-03).
//  4. Enqueues the outbox entry for delivery (FR-ING-02, RULE-EVT-02).
//  5. Commits the transaction. On any error, rolls back.
//
// Context is propagated to all I/O calls (RULE-RES-01).
func (s *eventIngestionService) IngestEvent(ctx context.Context, tenantID string, req *dto.IngestEventRequest) (string, error) {
	// --- Input validation ---
	if req == nil {
		return "", fmt.Errorf("%w: request body is nil", domain.ErrInvalidInput)
	}
	if tenantID == "" {
		return "", fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidInput)
	}
	if req.EventType == "" {
		return "", fmt.Errorf("%w: event_type is required", domain.ErrInvalidInput)
	}
	if len(req.Payload) == 0 {
		return "", fmt.Errorf("%w: payload is required", domain.ErrInvalidInput)
	}
	if req.DedupKey == "" {
		return "", fmt.Errorf("%w: dedup_key is required", domain.ErrInvalidInput)
	}

	now := s.clock()

	eventID := uuid.New().String()
	event := &domain.Event{
		ID:        eventID,
		TenantID:  tenantID,
		EventType: req.EventType,
		Payload:   req.Payload,
		DedupKey:  req.DedupKey,
		CreatedAt: now,
		CreatedBy: tenantID, // In a real system this comes from auth context (ASSUMP-01)
	}

	var resultID string

	// Execute within atomic transaction (RULE-DATA-01, RULE-EVT-02)
	err := s.txManager.ExecInTx(ctx, func(txCtx context.Context, tx interface{}) error {
		// Step 1: Persist the event
		createdID, err := s.eventRepo.CreateEvent(txCtx, event)
		if err != nil {
			if errors.Is(err, domain.ErrEventAlreadyExists) {
				// Idempotent case (FR-ING-03): the event already exists.
				// We still need to enqueue an outbox entry for this event delivery.
				resultID = createdID

				// Enqueue an outbox entry for delivery using the existing event
				outboxEntry := buildOutboxEntry(createdID, now)
				_, enqErr := s.outboxRepo.EnqueueEvent(txCtx, outboxEntry)
				if enqErr != nil {
					return fmt.Errorf("enqueue outbox for existing event: %w", enqErr)
				}
				return nil // commit — the event exists, outbox entry was created
			}
			// Genuine persistence error
			return fmt.Errorf("create event: %w", err)
		}

		resultID = createdID

		// Step 2: Enqueue outbox entry for delivery (RULE-EVT-02: transactional outbox)
		outboxEntry := buildOutboxEntry(createdID, now)
		_, enqErr := s.outboxRepo.EnqueueEvent(txCtx, outboxEntry)
		if enqErr != nil {
			return fmt.Errorf("enqueue outbox: %w", enqErr)
		}

		return nil // commit
	})

	if err != nil {
		return "", err
	}

	return resultID, nil
}

// buildOutboxEntry creates an OutboxEntry for a given event ID.
// In a full system this would also resolve matching subscriptions; for now
// we create a minimal entry that the dispatcher (ST-05) will process later.
func buildOutboxEntry(eventID string, now time.Time) *domain.OutboxEntry {
	return &domain.OutboxEntry{
		ID:             uuid.New().String(),
		EventID:        eventID,
		SubscriptionID: "", // Resolved by dispatcher at delivery time
		Status:         "pending",
		Attempts:       0,
		NextAttemptAt:  now,
		DeliveryID:     uuid.New().String(),
		CreatedAt:      now,
	}
}
