package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/repository"
)

const maxPayloadSize = 256 * 1024

type DBPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type IngestService struct {
	eventRepo repository.EventRepository
	subRepo   repository.SubscriptionRepository
	db        DBPool
	clock     domain.Clock
}

func NewIngestService(
	eventRepo repository.EventRepository,
	subRepo repository.SubscriptionRepository,
	db DBPool,
	clock domain.Clock,
) *IngestService {
	return &IngestService{
		eventRepo: eventRepo,
		subRepo:   subRepo,
		db:        db,
		clock:     clock,
	}
}

func (s *IngestService) Ingest(
	ctx context.Context,
	tenantID, eventType string,
	payload []byte,
	dedupKey string,
) (*domain.Event, error) {
	if tenantID == "" {
		return nil, domain.ErrTenantMissing
	}
	if eventType == "" || len(eventType) > 256 {
		return nil, fmt.Errorf("%w: event_type required, max 256 chars", domain.ErrInvalidEventPayload)
	}
	if dedupKey == "" {
		return nil, fmt.Errorf("%w: dedup_key required", domain.ErrInvalidEventPayload)
	}
	if len(payload) > maxPayloadSize {
		return nil, fmt.Errorf("%w: payload exceeds 256KB", domain.ErrInvalidEventPayload)
	}

	now := s.clock.Now()
	evt := &domain.Event{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		EventType: eventType,
		Payload:   payload,
		DedupKey:  dedupKey,
		CreatedAt: now,
		CreatedBy: "system",
	}

	entry := &domain.OutboxEntry{
		ID:             uuid.New().String(),
		EventID:        evt.ID,
		SubscriptionID: "",
		Status:         domain.DeliveryStatusPending,
		Attempts:       0,
		NextAttemptAt:  now,
		DeliveryID:     uuid.New().String(),
		CreatedAt:      now,
		CreatedBy:      "system",
	}

	// RULE-DATA-01: event + outbox in single transaction.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // RULE-RES-03: immediate defer

	if err := s.eventRepo.CreateWithOutbox(ctx, tx, evt, []*domain.OutboxEntry{entry}); err != nil {
		// RULE-DATA-02: detect unique violation → return existing event.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if existing, findErr := s.eventRepo.FindByDedupKey(ctx, tenantID, dedupKey); findErr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("persist event+outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return evt, nil
}
