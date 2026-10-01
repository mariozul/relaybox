package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/domain"
)

// PostgresOutboxRepository manages the transactional outbox via pgx.
type PostgresOutboxRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresOutboxRepository creates a PostgresOutboxRepository.
func NewPostgresOutboxRepository(pool *pgxpool.Pool) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{pool: pool}
}

// CreateBatch inserts multiple outbox messages. Should be called within a transaction.
func (r *PostgresOutboxRepository) CreateBatch(ctx context.Context, messages []domain.OutboxMessage) error {
	if len(messages) == 0 {
		return nil
	}

	query := `
		INSERT INTO outbox (event_id, subscription_id, status, attempts, next_attempt_at, delivery_id, created_by, created_at, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	batch := &pgx.Batch{}
	for _, m := range messages {
		batch.Queue(query,
			m.EventID, m.SubscriptionID, m.Status, m.Attempts, m.NextAttemptAt,
			m.DeliveryID, m.CreatedBy, m.CreatedAt, m.UpdatedBy, m.UpdatedAt,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range messages {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("pgx outbox batch insert: %w", err)
		}
	}
	return nil
}

// ClaimPending locks and returns up to limit pending outbox rows ordered by next_attempt_at.
func (r *PostgresOutboxRepository) ClaimPending(ctx context.Context, limit int) ([]domain.OutboxMessage, error) {
	query := `
		SELECT id, event_id, subscription_id, status, attempts, next_attempt_at, delivery_id,
		       created_by, created_at, updated_by, updated_at
		FROM outbox
		WHERE status = 'pending' AND next_attempt_at <= now()
		ORDER BY next_attempt_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("pgx outbox claim: %w", err)
	}
	defer rows.Close()

	var result []domain.OutboxMessage
	for rows.Next() {
		var m domain.OutboxMessage
		if err := rows.Scan(
			&m.ID, &m.EventID, &m.SubscriptionID, &m.Status, &m.Attempts,
			&m.NextAttemptAt, &m.DeliveryID,
			&m.CreatedBy, &m.CreatedAt, &m.UpdatedBy, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("pgx outbox scan: %w", err)
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgx outbox rows: %w", err)
	}
	return result, nil
}

// MarkDelivered updates an outbox row as delivered.
func (r *PostgresOutboxRepository) MarkDelivered(ctx context.Context, id string) error {
	query := `
		UPDATE outbox SET status = 'delivered', updated_at = now()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("pgx outbox mark delivered: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkRetry increments attempts and sets next_attempt_at for retry.
func (r *PostgresOutboxRepository) MarkRetry(ctx context.Context, id string, nextAttemptAt time.Time) error {
	query := `
		UPDATE outbox SET attempts = attempts + 1, next_attempt_at = $2, updated_at = now()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, query, id, nextAttemptAt)
	if err != nil {
		return fmt.Errorf("pgx outbox mark retry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeadLetter marks an outbox row as permanently failed.
func (r *PostgresOutboxRepository) DeadLetter(ctx context.Context, id string) error {
	query := `
		UPDATE outbox SET status = 'deadletter', updated_at = now()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("pgx outbox deadletter: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
