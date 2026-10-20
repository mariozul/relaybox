package postgresql

import (
	"context"
	"database/sql"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// pgOutboxRepository implements repository.OutboxRepository using PostgreSQL.
// It supports the transactional outbox pattern (FR-ING-02, RULE-EVT-02) for
// reliable at-least-once event delivery.
type pgOutboxRepository struct {
	db *sql.DB
}

// NewOutboxRepository creates a new PostgreSQL-backed OutboxRepository.
func NewOutboxRepository(db *sql.DB) *pgOutboxRepository {
	return &pgOutboxRepository{db: db}
}

// compile-time interface compliance check.
var _ interface {
	Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error)
	FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error)
	UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error
} = (*pgOutboxRepository)(nil)

// Save persists a new outbox entry. This is typically called within the same
// transaction as EventRepository.Save to guarantee atomicity (RULE-DATA-01).
// If called standalone, the entry is independently committed.
func (r *pgOutboxRepository) Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error) {
	query := `
		INSERT INTO outbox (id, event_id, subscription_id, status, attempts, next_attempt_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, event_id, subscription_id, status, attempts, next_attempt_at, created_at
	`

	saved := &domain.OutboxEntry{}
	err := r.db.QueryRowContext(ctx, query,
		entry.ID, entry.EventID, entry.SubscriptionID, entry.Status,
		entry.Attempts, entry.NextAttemptAt, entry.CreatedAt,
	).Scan(&saved.ID, &saved.EventID, &saved.SubscriptionID, &saved.Status,
		&saved.Attempts, &saved.NextAttemptAt, &saved.CreatedAt)

	if err != nil {
		return nil, domain.ErrInternal
	}

	return saved, nil
}

// FindPending retrieves pending outbox entries (status = 'pending' or 'failed'),
// up to the specified limit. Entries are ordered by next_attempt_at ascending
// (NULLS FIRST) to prioritize entries that haven't been retried or are due for
// retry. Returns an empty slice (not nil) if no pending entries exist.
func (r *pgOutboxRepository) FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	query := `
		SELECT id, event_id, subscription_id, status, attempts, next_attempt_at, created_at
		FROM outbox
		WHERE status IN ($1, $2)
		ORDER BY next_attempt_at NULLS FIRST, created_at ASC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query,
		domain.OutboxStatusPending, domain.OutboxStatusFailed, limit,
	)
	if err != nil {
		return nil, domain.ErrInternal
	}
	// RULE-RES-03: immediate deferral for resource cleanup.
	defer func() {
		_ = rows.Close()
	}()

	var entries []domain.OutboxEntry
	for rows.Next() {
		var entry domain.OutboxEntry
		if err := rows.Scan(&entry.ID, &entry.EventID, &entry.SubscriptionID,
			&entry.Status, &entry.Attempts, &entry.NextAttemptAt, &entry.CreatedAt); err != nil {
			return nil, domain.ErrInternal
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, domain.ErrInternal
	}

	// Return empty slice instead of nil.
	if entries == nil {
		entries = make([]domain.OutboxEntry, 0)
	}

	return entries, nil
}

// UpdateStatus updates the delivery status of an outbox entry.
// Returns domain.ErrNotFound if the entry does not exist.
func (r *pgOutboxRepository) UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error {
	query := `
		UPDATE outbox
		SET status = $1
		WHERE id = $2
	`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return domain.ErrInternal
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return domain.ErrInternal
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// DBChecker provides a simple health check for PostgreSQL reachability.
// It is used by the /readyz endpoint to determine if the service is ready
// to accept traffic (FR-OBS-01, RULE-OBS-04).
type DBChecker struct {
	db *sql.DB
}

// NewDBChecker creates a new DBChecker for health probe purposes.
func NewDBChecker(db *sql.DB) *DBChecker {
	return &DBChecker{db: db}
}

// CheckDB pings the database to verify connectivity. Returns an error if the
// database is unreachable or the ping times out. This is used by the /readyz
// handler to report 503 when the database is down (TC-08-EDGE).
func (c *DBChecker) CheckDB(ctx context.Context) error {
	if err := c.db.PingContext(ctx); err != nil {
		return domain.ErrInternal
	}
	return nil
}
