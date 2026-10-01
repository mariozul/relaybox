package postgres

import (
	"context"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

// PgOutboxStore implements domain.OutboxStore backed by PostgreSQL.
type PgOutboxStore struct {
	pool DBPool
}

// NewPgOutboxStore creates a new PgOutboxStore.
func NewPgOutboxStore(pool DBPool) *PgOutboxStore {
	return &PgOutboxStore{pool: pool}
}

// CreateOutbox inserts a pending outbox row (FR-ING-02, RULE-EVT-02).
func (s *PgOutboxStore) CreateOutbox(ctx context.Context, eventID, subscriptionID string) (domain.Outbox, error) {
	const query = `
		INSERT INTO outbox (id, event_id, subscription_id, status, next_attempt_at)
		VALUES (gen_random_uuid()::text, $1, $2, 'pending', NOW())
		RETURNING id, event_id, subscription_id, status, attempts, next_attempt_at, last_error, created_at, updated_at
	`

	var ob domain.Outbox
	err := s.pool.QueryRow(ctx, query, eventID, subscriptionID).Scan(
		&ob.ID, &ob.EventID, &ob.SubscriptionID, &ob.Status,
		&ob.Attempts, &ob.NextAttemptAt, &ob.LastError, &ob.CreatedAt, &ob.UpdatedAt,
	)
	if err != nil {
		return domain.Outbox{}, err
	}
	return ob, nil
}

// FetchPending retrieves pending outbox rows ordered by next_attempt_at (FR-DEL-01).
func (s *PgOutboxStore) FetchPending(ctx context.Context, limit int) ([]domain.Outbox, error) {
	const query = `
		SELECT id, event_id, subscription_id, status, attempts, next_attempt_at, last_error, created_at, updated_at
		FROM outbox
		WHERE status = 'pending' AND next_attempt_at <= NOW()
		ORDER BY next_attempt_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.Outbox
	for rows.Next() {
		var ob domain.Outbox
		if err := rows.Scan(
			&ob.ID, &ob.EventID, &ob.SubscriptionID, &ob.Status,
			&ob.Attempts, &ob.NextAttemptAt, &ob.LastError, &ob.CreatedAt, &ob.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, ob)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// MarkDelivered updates outbox status to 'delivered' (FR-DEL-02).
func (s *PgOutboxStore) MarkDelivered(ctx context.Context, id string) error {
	const query = `UPDATE outbox SET status = 'delivered', updated_at = NOW() WHERE id = $1`
	tag, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkFailed updates outbox status to 'failed' and sets next attempt time (FR-DEL-02).
func (s *PgOutboxStore) MarkFailed(ctx context.Context, id string, errMsg string, nextAttemptAt time.Time) error {
	const query = `
		UPDATE outbox
		SET status = 'failed', attempts = attempts + 1, last_error = $2,
			next_attempt_at = $3, updated_at = NOW()
		WHERE id = $1`

	tag, err := s.pool.Exec(ctx, query, id, errMsg, nextAttemptAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkDeadLetter moves outbox to dead_letter status (FR-DEL-02).
func (s *PgOutboxStore) MarkDeadLetter(ctx context.Context, id string, errMsg string) error {
	const query = `
		UPDATE outbox
		SET status = 'dead_letter', last_error = $2, updated_at = NOW()
		WHERE id = $1`

	tag, err := s.pool.Exec(ctx, query, id, errMsg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
