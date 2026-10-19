package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// compile-time check.
var _ repository.OutboxRepository = (*postgresOutboxRepository)(nil)

type postgresOutboxRepository struct {
	db *sql.DB
}

// NewPostgresOutboxRepository creates a new PostgreSQL-backed OutboxRepository.
func NewPostgresOutboxRepository(db *sql.DB) repository.OutboxRepository {
	return &postgresOutboxRepository{db: db}
}

// EnqueueEvent inserts a new outbox entry for delivery (FR-ING-02).
func (r *postgresOutboxRepository) EnqueueEvent(ctx context.Context, entry *domain.OutboxEntry) (string, error) {
	query := `
		INSERT INTO outbox (id, event_id, subscription_id, status, attempts, next_attempt_at, delivery_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		entry.ID,
		entry.EventID,
		entry.SubscriptionID,
		entry.Status,
		entry.Attempts,
		entry.NextAttemptAt,
		entry.DeliveryID,
		entry.CreatedAt,
	)
	if err != nil {
		return "", err
	}
	return entry.ID, nil
}

// GetPendingOutboxEntries retrieves entries ready for delivery, limited by count.
func (r *postgresOutboxRepository) GetPendingOutboxEntries(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	query := `
		SELECT id, event_id, subscription_id, status, attempts, next_attempt_at,
		       COALESCE(last_attempt_at, '0001-01-01T00:00:00Z'), COALESCE(last_attempt_error, ''),
		       delivery_id, created_at
		FROM outbox
		WHERE status = 'pending' AND next_attempt_at <= NOW()
		ORDER BY next_attempt_at ASC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []domain.OutboxEntry
	for rows.Next() {
		var e domain.OutboxEntry
		err := rows.Scan(
			&e.ID,
			&e.EventID,
			&e.SubscriptionID,
			&e.Status,
			&e.Attempts,
			&e.NextAttemptAt,
			&e.LastAttemptAt,
			&e.LastAttemptError,
			&e.DeliveryID,
			&e.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []domain.OutboxEntry{}
	}
	return entries, nil
}

// MarkOutboxEntryProcessed updates the status and attempt information for an outbox entry.
func (r *postgresOutboxRepository) MarkOutboxEntryProcessed(
	ctx context.Context,
	id string,
	status string,
	attempts int,
	nextAttemptAt *time.Time,
	lastAttemptError *string,
) error {
	query := `
		UPDATE outbox
		SET status = $1, attempts = $2, next_attempt_at = $3,
		    last_attempt_at = NOW(), last_attempt_error = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query,
		status,
		attempts,
		nextAttemptAt,
		lastAttemptError,
		id,
	)
	return err
}

// ============================================================
// Postgres Transaction Manager
// ============================================================

// compile-time check.
var _ repository.TransactionManager = (*postgresTransactionManager)(nil)

type postgresTransactionManager struct {
	db *sql.DB
}

// NewPostgresTransactionManager creates a new PostgreSQL-backed TransactionManager.
func NewPostgresTransactionManager(db *sql.DB) repository.TransactionManager {
	return &postgresTransactionManager{db: db}
}

// BeginTx starts a new database transaction (RULE-DATA-01).
func (m *postgresTransactionManager) BeginTx(ctx context.Context) (interface{}, error) {
	return m.db.BeginTx(ctx, nil)
}

// CommitTx commits the provided database transaction.
func (m *postgresTransactionManager) CommitTx(ctx context.Context, tx interface{}) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return domain.ErrInternal
	}
	return sqlTx.Commit()
}

// RollbackTx rolls back the provided database transaction.
func (m *postgresTransactionManager) RollbackTx(ctx context.Context, tx interface{}) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return domain.ErrInternal
	}
	return sqlTx.Rollback()
}

// ExecInTx executes a function within a transaction, automatically rolling back on error (RULE-DATA-01).
func (m *postgresTransactionManager) ExecInTx(ctx context.Context, fn func(txCtx context.Context, tx interface{}) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	err = fn(ctx, tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return err
		}
		return err
	}

	return tx.Commit()
}
