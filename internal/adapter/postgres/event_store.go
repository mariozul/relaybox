package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mariozul/relaybox/internal/domain"
)

// PgEventStore implements domain.EventStore backed by PostgreSQL (pgx).
type PgEventStore struct {
	pool DBPool
}

// NewPgEventStore creates a new PgEventStore.
func NewPgEventStore(pool DBPool) *PgEventStore {
	return &PgEventStore{pool: pool}
}

// DBPool is the minimal pgx interface for querying.
type DBPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// CreateEvent inserts an event with a unique (tenant_id, dedup_key) constraint (FR-ING-03, RULE-DATA-02).
// Returns the event. If a conflict occurs (duplicate dedup key), returns the existing event.
func (s *PgEventStore) CreateEvent(ctx context.Context, tenantID, eventType string, payload []byte, dedupKey string) (domain.Event, error) {
	const query = `
		INSERT INTO events (id, tenant_id, event_type, payload, dedup_key, created_by)
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, 'system')
		ON CONFLICT (tenant_id, dedup_key) DO UPDATE SET tenant_id = EXCLUDED.tenant_id
		RETURNING id, tenant_id, event_type, payload, dedup_key, created_at, created_by
	`

	var evt domain.Event
	err := s.pool.QueryRow(ctx, query, tenantID, eventType, payload, dedupKey).Scan(
		&evt.ID, &evt.TenantID, &evt.EventType, &evt.Payload, &evt.DedupKey, &evt.CreatedAt, &evt.CreatedBy,
	)
	if err != nil {
		return domain.Event{}, err
	}
	return evt, nil
}

// GetEvent retrieves an event by ID.
func (s *PgEventStore) GetEvent(ctx context.Context, id string) (domain.Event, error) {
	const query = `SELECT id, tenant_id, event_type, payload, dedup_key, created_at, created_by
		FROM events WHERE id = $1`

	var evt domain.Event
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&evt.ID, &evt.TenantID, &evt.EventType, &evt.Payload, &evt.DedupKey, &evt.CreatedAt, &evt.CreatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, err
	}
	return evt, nil
}

// GetEventByDedup retrieves an event by tenant_id + dedup_key for idempotency.
func (s *PgEventStore) GetEventByDedup(ctx context.Context, tenantID, dedupKey string) (domain.Event, error) {
	const query = `SELECT id, tenant_id, event_type, payload, dedup_key, created_at, created_by
		FROM events WHERE tenant_id = $1 AND dedup_key = $2`

	var evt domain.Event
	err := s.pool.QueryRow(ctx, query, tenantID, dedupKey).Scan(
		&evt.ID, &evt.TenantID, &evt.EventType, &evt.Payload, &evt.DedupKey, &evt.CreatedAt, &evt.CreatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, err
	}
	return evt, nil
}
