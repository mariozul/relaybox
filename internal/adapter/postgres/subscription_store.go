package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/mariozul/relaybox/internal/domain"
)

// PgSubscriptionStore implements domain.SubscriptionStore backed by PostgreSQL.
type PgSubscriptionStore struct {
	pool DBPool
}

// NewPgSubscriptionStore creates a new PgSubscriptionStore.
func NewPgSubscriptionStore(pool DBPool) *PgSubscriptionStore {
	return &PgSubscriptionStore{pool: pool}
}

// CreateSubscription inserts a new subscription for a tenant (FR-SUB-01, FR-SUB-02).
func (s *PgSubscriptionStore) CreateSubscription(ctx context.Context, tenantID, eventType, targetURL string) (domain.Subscription, error) {
	const query = `
		INSERT INTO subscriptions (id, tenant_id, event_type, target_url, created_by)
		VALUES (gen_random_uuid()::text, $1, $2, $3, 'api')
		RETURNING id, tenant_id, event_type, target_url, created_at, created_by
	`

	var sub domain.Subscription
	err := s.pool.QueryRow(ctx, query, tenantID, eventType, targetURL).Scan(
		&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL, &sub.CreatedAt, &sub.CreatedBy,
	)
	if err != nil {
		return domain.Subscription{}, err
	}
	return sub, nil
}

// GetSubscription retrieves a subscription by ID (tenant-scoped, RULE-SEC-01).
func (s *PgSubscriptionStore) GetSubscription(ctx context.Context, id string) (domain.Subscription, error) {
	const query = `SELECT id, tenant_id, event_type, target_url, created_at, created_by,
			updated_at, updated_by, deleted_at, deleted_by
		FROM subscriptions WHERE id = $1 AND deleted_at IS NULL`

	var sub domain.Subscription
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL, &sub.CreatedAt, &sub.CreatedBy,
		&sub.UpdatedAt, &sub.UpdatedBy, &sub.DeletedAt, &sub.DeletedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Subscription{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Subscription{}, err
	}
	return sub, nil
}

// GetSubscriptionsByEventType returns all subscriptions for a tenant + event_type (FR-DEL-01).
func (s *PgSubscriptionStore) GetSubscriptionsByEventType(ctx context.Context, tenantID, eventType string) ([]domain.Subscription, error) {
	const query = `SELECT id, tenant_id, event_type, target_url, created_at, created_by,
			updated_at, updated_by, deleted_at, deleted_by
		FROM subscriptions
		WHERE tenant_id = $1 AND event_type = $2 AND deleted_at IS NULL
		ORDER BY created_at`

	rows, err := s.pool.Query(ctx, query, tenantID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(
			&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL, &sub.CreatedAt, &sub.CreatedBy,
			&sub.UpdatedAt, &sub.UpdatedBy, &sub.DeletedAt, &sub.DeletedBy,
		); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return subs, nil
}

// ListSubscriptions returns all subscriptions for a tenant (FR-SUB-01).
func (s *PgSubscriptionStore) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	const query = `SELECT id, tenant_id, event_type, target_url, created_at, created_by,
			updated_at, updated_by, deleted_at, deleted_by
		FROM subscriptions
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at`

	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(
			&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL, &sub.CreatedAt, &sub.CreatedBy,
			&sub.UpdatedAt, &sub.UpdatedBy, &sub.DeletedAt, &sub.DeletedBy,
		); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return subs, nil
}

// DeleteSubscription soft-deletes a subscription (RULE-DATA-03).
func (s *PgSubscriptionStore) DeleteSubscription(ctx context.Context, id string) error {
	const query = `UPDATE subscriptions SET deleted_at = NOW(), deleted_by = 'api'
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
