package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// pgSubscriptionRepository implements repository.SubscriptionRepository using
// PostgreSQL. All queries are scoped by tenantID to enforce strict tenant
// isolation (FR-SUB-02, RULE-SEC-01).
type pgSubscriptionRepository struct {
	db *sql.DB
}

// NewSubscriptionRepository creates a new PostgreSQL-backed SubscriptionRepository.
func NewSubscriptionRepository(db *sql.DB) *pgSubscriptionRepository {
	return &pgSubscriptionRepository{db: db}
}

// compile-time interface compliance check.
var _ interface {
	Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)
	FindByID(ctx context.Context, tenantID string, id string) (*domain.Subscription, error)
	FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error)
	Delete(ctx context.Context, tenantID string, id string) error
} = (*pgSubscriptionRepository)(nil)

// Save creates or updates a subscription. Returns the saved copy.
func (r *pgSubscriptionRepository) Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	query := `
		INSERT INTO subscriptions (id, tenant_id, event_type, target_url, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $5, $6, $6)
		RETURNING id, tenant_id, event_type, target_url, created_at, created_by
	`

	saved := &domain.Subscription{}
	err := r.db.QueryRowContext(ctx, query,
		sub.ID, sub.TenantID, sub.EventType, sub.TargetURL,
		sub.CreatedAt, sub.CreatedBy,
	).Scan(&saved.ID, &saved.TenantID, &saved.EventType, &saved.TargetURL,
		&saved.CreatedAt, &saved.CreatedBy)

	if err != nil {
		return nil, domain.ErrInternal
	}

	return saved, nil
}

// FindByID retrieves a subscription by ID, scoped to the given tenant.
// Returns domain.ErrSubscriptionNotFound if the subscription does not exist
// or is owned by a different tenant (RULE-SEC-01).
func (r *pgSubscriptionRepository) FindByID(ctx context.Context, tenantID string, id string) (*domain.Subscription, error) {
	query := `
		SELECT id, tenant_id, event_type, target_url, created_at, created_by
		FROM subscriptions
		WHERE id = $1 AND tenant_id = $2
	`

	sub := &domain.Subscription{}
	err := r.db.QueryRowContext(ctx, query, id, tenantID).
		Scan(&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL,
			&sub.CreatedAt, &sub.CreatedBy)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, domain.ErrInternal
	}

	return sub, nil
}

// FindByTenantID returns all subscriptions owned by the specified tenant.
func (r *pgSubscriptionRepository) FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	query := `
		SELECT id, tenant_id, event_type, target_url, created_at, created_by
		FROM subscriptions
		WHERE tenant_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, domain.ErrInternal
	}
	// RULE-RES-03: immediate deferral for resource cleanup.
	defer func() {
		_ = rows.Close()
	}()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(&sub.ID, &sub.TenantID, &sub.EventType, &sub.TargetURL,
			&sub.CreatedAt, &sub.CreatedBy); err != nil {
			return nil, domain.ErrInternal
		}
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, domain.ErrInternal
	}

	// Return empty slice instead of nil for consistent API.
	if subs == nil {
		subs = make([]domain.Subscription, 0)
	}

	return subs, nil
}

// Delete removes a subscription by ID, scoped to the given tenant.
// Returns domain.ErrSubscriptionNotFound if the subscription does not exist
// or is owned by a different tenant (RULE-SEC-01).
func (r *pgSubscriptionRepository) Delete(ctx context.Context, tenantID string, id string) error {
	// Use current_timestamp for updated_at to track deletion time.
	query := `
		DELETE FROM subscriptions
		WHERE id = $1 AND tenant_id = $2
		RETURNING id
	`

	var deletedID string
	err := r.db.QueryRowContext(ctx, query, id, tenantID).Scan(&deletedID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return domain.ErrInternal
	}

	return nil
}

// UpdateUpdatedAt is a helper to update the updated_at and updated_by columns
// on a subscription. This is useful when side effects need to track mutation.
func (r *pgSubscriptionRepository) UpdateUpdatedAt(ctx context.Context, id, updatedBy string, now time.Time) error {
	query := `UPDATE subscriptions SET updated_at = $1, updated_by = $2 WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, now, updatedBy, id)
	if err != nil {
		return domain.ErrInternal
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return domain.ErrInternal
	}
	if rows == 0 {
		return domain.ErrSubscriptionNotFound
	}

	return nil
}
