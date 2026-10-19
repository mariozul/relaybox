package repository

import (
	"context"
	"database/sql"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// compile-time check that postgresSubscriptionRepository satisfies repository.SubscriptionRepository.
var _ repository.SubscriptionRepository = (*postgresSubscriptionRepository)(nil)

type postgresSubscriptionRepository struct {
	db *sql.DB
}

// NewPostgresSubscriptionRepository creates a new PostgreSQL-backed SubscriptionRepository.
func NewPostgresSubscriptionRepository(db *sql.DB) repository.SubscriptionRepository {
	return &postgresSubscriptionRepository{db: db}
}

// CreateSubscription inserts a new subscription record (FR-SUB-01).
func (r *postgresSubscriptionRepository) CreateSubscription(ctx context.Context, sub *domain.Subscription) (string, error) {
	query := `
		INSERT INTO subscriptions (id, tenant_id, event_type, target_url, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		sub.ID,
		sub.TenantID,
		sub.EventType,
		sub.TargetURL,
		sub.CreatedAt,
		sub.CreatedBy,
		sub.UpdatedAt,
		sub.UpdatedBy,
	)
	if err != nil {
		return "", err
	}
	return sub.ID, nil
}

// GetSubscriptionByID retrieves a subscription scoped to the requesting tenant (FR-SUB-02).
// Returns nil, nil if not found or not owned by tenant.
func (r *postgresSubscriptionRepository) GetSubscriptionByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	query := `
		SELECT id, tenant_id, event_type, target_url, created_at, created_by, updated_at, updated_by
		FROM subscriptions
		WHERE id = $1 AND tenant_id = $2
	`
	sub := &domain.Subscription{}
	err := r.db.QueryRowContext(ctx, query, id, tenantID).Scan(
		&sub.ID,
		&sub.TenantID,
		&sub.EventType,
		&sub.TargetURL,
		&sub.CreatedAt,
		&sub.CreatedBy,
		&sub.UpdatedAt,
		&sub.UpdatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// ListSubscriptions retrieves all subscriptions for a given tenant (FR-SUB-01, FR-SUB-02).
func (r *postgresSubscriptionRepository) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	query := `
		SELECT id, tenant_id, event_type, target_url, created_at, created_by, updated_at, updated_by
		FROM subscriptions
		WHERE tenant_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		err := rows.Scan(
			&sub.ID,
			&sub.TenantID,
			&sub.EventType,
			&sub.TargetURL,
			&sub.CreatedAt,
			&sub.CreatedBy,
			&sub.UpdatedAt,
			&sub.UpdatedBy,
		)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if subs == nil {
		subs = []domain.Subscription{}
	}
	return subs, nil
}

// UpdateSubscription modifies an existing subscription (FR-SUB-01).
// Returns domain.ErrSubscriptionNotFound if no row matched the id and tenant.
func (r *postgresSubscriptionRepository) UpdateSubscription(ctx context.Context, sub *domain.Subscription) error {
	query := `
		UPDATE subscriptions
		SET event_type = $1, target_url = $2, updated_at = $3, updated_by = $4
		WHERE id = $5 AND tenant_id = $6
	`
	result, err := r.db.ExecContext(ctx, query,
		sub.EventType,
		sub.TargetURL,
		sub.UpdatedAt,
		sub.UpdatedBy,
		sub.ID,
		sub.TenantID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrSubscriptionNotFound
	}
	return nil
}

// DeleteSubscription removes a subscription scoped to tenant (FR-SUB-01, FR-SUB-02).
// Returns domain.ErrSubscriptionNotFound if no row matched.
func (r *postgresSubscriptionRepository) DeleteSubscription(ctx context.Context, tenantID, id string) error {
	query := `DELETE FROM subscriptions WHERE id = $1 AND tenant_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, tenantID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrSubscriptionNotFound
	}
	return nil
}
