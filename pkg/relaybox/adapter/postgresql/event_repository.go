// Package postgresql provides concrete PostgreSQL implementations of the
// repository interfaces defined in the repository package. It implements the
// Transactional Outbox Pattern (RULE-EVT-02) for reliable event persistence
// and delivery.
package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// pgEventRepository implements repository.EventRepository using PostgreSQL.
// It provides atomic event persistence with integrated outbox enqueueing
// within a single database transaction (FR-ING-02, RULE-DATA-01).
type pgEventRepository struct {
	db *sql.DB
}

// NewEventRepository creates a new PostgreSQL-backed EventRepository.
func NewEventRepository(db *sql.DB) *pgEventRepository {
	return &pgEventRepository{db: db}
}

// compile-time interface compliance check.
var _ interface {
	Save(ctx context.Context, event *domain.Event) (*domain.Event, error)
	FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error)
	SaveWithOutbox(ctx context.Context, event *domain.Event, subscriptionID string, now time.Time) (*domain.Event, error)
} = (*pgEventRepository)(nil)

// Save persists a new event. It does NOT create an outbox entry; use
// SaveWithOutbox for transactional event + outbox persistence.
func (r *pgEventRepository) Save(ctx context.Context, event *domain.Event) (*domain.Event, error) {
	query := `
		INSERT INTO events (id, tenant_id, event_type, payload, dedup_key, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6, $7, $7)
		RETURNING id, tenant_id, event_type, payload, dedup_key, created_at, created_by
	`

	saved := &domain.Event{}
	err := r.db.QueryRowContext(ctx, query,
		event.ID, event.TenantID, event.EventType, event.Payload,
		event.DedupKey, event.CreatedAt, event.CreatedBy,
	).Scan(&saved.ID, &saved.TenantID, &saved.EventType, &saved.Payload,
		&saved.DedupKey, &saved.CreatedAt, &saved.CreatedBy)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			// Unique constraint violation on (tenant_id, dedup_key).
			return nil, domain.ErrDuplicateEvent
		}
		return nil, domain.ErrInternal
	}

	return saved, nil
}

// FindByDedupKey retrieves an event by tenant ID and dedup key.
// Returns domain.ErrNotFound if no matching event exists.
func (r *pgEventRepository) FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error) {
	query := `
		SELECT id, tenant_id, event_type, payload, dedup_key, created_at, created_by
		FROM events
		WHERE tenant_id = $1 AND dedup_key = $2
	`

	event := &domain.Event{}
	err := r.db.QueryRowContext(ctx, query, tenantID, dedupKey).
		Scan(&event.ID, &event.TenantID, &event.EventType, &event.Payload,
			&event.DedupKey, &event.CreatedAt, &event.CreatedBy)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, domain.ErrInternal
	}

	return event, nil
}

// SaveWithOutbox atomically persists an event and creates an outbox entry
// within a single database transaction (FR-ING-02, RULE-DATA-01, RULE-EVT-02).
// This guarantees that event persistence and delivery enqueueing are atomic:
// if the transaction commits, both the event and the outbox entry are persisted;
// if it fails, neither is created.
func (r *pgEventRepository) SaveWithOutbox(ctx context.Context, event *domain.Event, subscriptionID string, now time.Time) (*domain.Event, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, domain.ErrInternal
	}
	// RULE-RES-03: defer rollback on error; if committed, rollback is a no-op.
	defer func() {
		_ = tx.Rollback()
	}()

	// Step 1: Insert the event.
	eventQuery := `
		INSERT INTO events (id, tenant_id, event_type, payload, dedup_key, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6, $7, $7)
		RETURNING id, tenant_id, event_type, payload, dedup_key, created_at, created_by
	`

	saved := &domain.Event{}
	err = tx.QueryRowContext(ctx, eventQuery,
		event.ID, event.TenantID, event.EventType, event.Payload,
		event.DedupKey, now, event.CreatedBy,
	).Scan(&saved.ID, &saved.TenantID, &saved.EventType, &saved.Payload,
		&saved.DedupKey, &saved.CreatedAt, &saved.CreatedBy)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicateEvent
		}
		return nil, domain.ErrInternal
	}

	// Step 2: Insert the outbox entry within the same transaction.
	outboxQuery := `
		INSERT INTO outbox (id, event_id, subscription_id, status, attempts, next_attempt_at, created_at)
		VALUES ($1, $2, $3, $4, 0, NULL, $5)
	`

	entryID := domain.GenerateDeliveryID(saved.ID, subscriptionID)
	_, err = tx.ExecContext(ctx, outboxQuery,
		entryID, saved.ID, subscriptionID, domain.OutboxStatusPending, now,
	)
	if err != nil {
		return nil, domain.ErrInternal
	}

	// Step 3: Commit the transaction.
	if err := tx.Commit(); err != nil {
		return nil, domain.ErrInternal
	}

	return saved, nil
}
