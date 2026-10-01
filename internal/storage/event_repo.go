package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/domain"
)

// PostgresEventRepository persists events via pgx.
type PostgresEventRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresEventRepository creates a PostgresEventRepository.
func NewPostgresEventRepository(pool *pgxpool.Pool) *PostgresEventRepository {
	return &PostgresEventRepository{pool: pool}
}

// CreateOrGet inserts an event or returns the existing one on (tenant_id, dedup_key) conflict.
func (r *PostgresEventRepository) CreateOrGet(ctx context.Context, event domain.Event) (*domain.Event, error) {
	query := `
		INSERT INTO events (tenant_id, event_type, payload, dedup_key, created_by, created_at, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, dedup_key)
		DO UPDATE SET updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by
		RETURNING id, tenant_id, event_type, payload, dedup_key, created_by, created_at, updated_by, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		event.TenantID, event.EventType, event.Payload, event.DedupKey,
		event.CreatedBy, event.CreatedAt, event.UpdatedBy, event.UpdatedAt,
	)

	var result domain.Event
	err := row.Scan(
		&result.ID, &result.TenantID, &result.EventType, &result.Payload,
		&result.DedupKey, &result.CreatedBy, &result.CreatedAt,
		&result.UpdatedBy, &result.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("pgx event insert: %w", err)
	}
	return &result, nil
}
