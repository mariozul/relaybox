package repository

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// compile-time check that postgresEventRepository satisfies repository.EventRepository.
var _ repository.EventRepository = (*postgresEventRepository)(nil)

type postgresEventRepository struct {
	db *sql.DB
}

// NewPostgresEventRepository creates a new PostgreSQL-backed EventRepository.
func NewPostgresEventRepository(db *sql.DB) repository.EventRepository {
	return &postgresEventRepository{db: db}
}

// CreateEvent inserts a new event row. Returns domain.ErrEventAlreadyExists if
// a row with the same (tenant_id, dedup_key) already exists. In that case it
// returns the existing row's ID (FR-ING-03 idempotent behavior).
func (r *postgresEventRepository) CreateEvent(ctx context.Context, event *domain.Event) (string, error) {
	query := `
		INSERT INTO events (id, tenant_id, event_type, payload, dedup_key, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.TenantID,
		event.EventType,
		event.Payload,
		event.DedupKey,
		event.CreatedAt,
		event.CreatedBy,
	)
	if err == nil {
		return event.ID, nil
	}

	// If unique violation (pq error code 23505), fetch existing (idempotent).
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
		existing, lookupErr := r.GetEventByDedupKey(ctx, event.TenantID, event.DedupKey)
		if lookupErr != nil {
			return "", lookupErr
		}
		if existing != nil {
			return existing.ID, domain.ErrEventAlreadyExists
		}
	}

	return "", err
}

// GetEventByDedupKey retrieves an event by tenant_id and dedup_key.
// Returns nil, nil if not found.
func (r *postgresEventRepository) GetEventByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	query := `
		SELECT id, tenant_id, event_type, payload, dedup_key, created_at, created_by
		FROM events
		WHERE tenant_id = $1 AND dedup_key = $2
	`
	event := &domain.Event{}
	err := r.db.QueryRowContext(ctx, query, tenantID, dedupKey).Scan(
		&event.ID,
		&event.TenantID,
		&event.EventType,
		&event.Payload,
		&event.DedupKey,
		&event.CreatedAt,
		&event.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}

// GetEventByID retrieves an event by its ID. Returns nil, nil if not found.
func (r *postgresEventRepository) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	query := `
		SELECT id, tenant_id, event_type, payload, dedup_key, created_at, created_by
		FROM events
		WHERE id = $1
	`
	event := &domain.Event{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.TenantID,
		&event.EventType,
		&event.Payload,
		&event.DedupKey,
		&event.CreatedAt,
		&event.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}
