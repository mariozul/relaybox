package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/domain"
)

// PostgresSubscriptionRepository manages tenant-scoped subscriptions via pgx.
type PostgresSubscriptionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresSubscriptionRepository creates a PostgresSubscriptionRepository.
func NewPostgresSubscriptionRepository(pool *pgxpool.Pool) *PostgresSubscriptionRepository {
	return &PostgresSubscriptionRepository{pool: pool}
}

// Create inserts a new subscription. Returns ErrConflict on duplicate.
func (r *PostgresSubscriptionRepository) Create(ctx context.Context, sub domain.Subscription) (*domain.Subscription, error) {
	query := `
		INSERT INTO subscriptions (tenant_id, event_type, target_url, created_by, created_at, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, event_type, target_url, created_by, created_at, updated_by, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		sub.TenantID, sub.EventType, sub.TargetURL,
		sub.CreatedBy, sub.CreatedAt, sub.UpdatedBy, sub.UpdatedAt,
	)

	var result domain.Subscription
	err := row.Scan(
		&result.ID, &result.TenantID, &result.EventType, &result.TargetURL,
		&result.CreatedBy, &result.CreatedAt, &result.UpdatedBy, &result.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("pgx sub insert: %w", err)
	}
	return &result, nil
}

// GetByID retrieves a subscription scoped to the given tenant.
func (r *PostgresSubscriptionRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	query := `
		SELECT id, tenant_id, event_type, target_url, created_by, created_at, updated_by, updated_at
		FROM subscriptions
		WHERE id = $1 AND tenant_id = $2
	`

	row := r.pool.QueryRow(ctx, query, id, tenantID)
	var result domain.Subscription
	err := row.Scan(
		&result.ID, &result.TenantID, &result.EventType, &result.TargetURL,
		&result.CreatedBy, &result.CreatedAt, &result.UpdatedBy, &result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pgx sub get: %w", err)
	}
	return &result, nil
}

// ListByTenant returns all subscriptions for a tenant.
func (r *PostgresSubscriptionRepository) ListByTenant(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	query := `
		SELECT id, tenant_id, event_type, target_url, created_by, created_at, updated_by, updated_at
		FROM subscriptions
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("pgx sub list: %w", err)
	}
	defer rows.Close()

	var result []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(
			&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL,
			&sub.CreatedBy, &sub.CreatedAt, &sub.UpdatedBy, &sub.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("pgx sub scan: %w", err)
		}
		result = append(result, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgx sub rows: %w", err)
	}
	return result, nil
}

// ListByEventType returns subscriptions for a tenant matching a specific event type.
func (r *PostgresSubscriptionRepository) ListByEventType(ctx context.Context, tenantID, eventType string) ([]domain.Subscription, error) {
	query := `
		SELECT id, tenant_id, event_type, target_url, created_by, created_at, updated_by, updated_at
		FROM subscriptions
		WHERE tenant_id = $1 AND event_type = $2
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, tenantID, eventType)
	if err != nil {
		return nil, fmt.Errorf("pgx sub list by type: %w", err)
	}
	defer rows.Close()

	var result []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(
			&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL,
			&sub.CreatedBy, &sub.CreatedAt, &sub.UpdatedBy, &sub.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("pgx sub scan: %w", err)
		}
		result = append(result, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgx sub rows: %w", err)
	}
	return result, nil
}

// Delete removes a subscription scoped to the given tenant.
func (r *PostgresSubscriptionRepository) Delete(ctx context.Context, tenantID, id string) error {
	query := `DELETE FROM subscriptions WHERE id = $1 AND tenant_id = $2`

	tag, err := r.pool.Exec(ctx, query, id, tenantID)
	if err != nil {
		return fmt.Errorf("pgx sub delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
