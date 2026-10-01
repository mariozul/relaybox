package adapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/domain"
)

// txKey is the context key for the current pgx transaction.
type txKey struct{}

// PGEventRepo implements EventRepository using pgx.
type PGEventRepo struct {
	pool *pgxpool.Pool
}

func NewPGEventRepo(pool *pgxpool.Pool) *PGEventRepo { return &PGEventRepo{pool: pool} }

func (r *PGEventRepo) getQuerier(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
} {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.pool
}

func (r *PGEventRepo) Create(ctx context.Context, event *domain.Event) (*domain.Event, error) {
	q := r.getQuerier(ctx)
	var id string
	var status string
	err := q.QueryRow(ctx,
		`INSERT INTO events (tenant_id, event_type, payload, dedup_key, status, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (tenant_id, dedup_key) DO NOTHING
		 RETURNING id, status`,
		event.TenantID, event.EventType, event.Payload, event.DedupKey, event.Status, event.CreatedBy,
	).Scan(&id, &status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			existing, err2 := r.FindByDedup(ctx, event.TenantID, event.DedupKey)
			if err2 != nil {
				return nil, err2
			}
			return existing, domain.ErrConflict
		}
		return nil, fmt.Errorf("pg: insert event: %w", err)
	}
	if id == "" {
		existing, err := r.FindByDedup(ctx, event.TenantID, event.DedupKey)
		if err != nil {
			return nil, err
		}
		return existing, domain.ErrConflict
	}
	event.ID = id
	return event, nil
}

func (r *PGEventRepo) FindByDedup(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, event_type, payload, dedup_key, status, created_at, created_by
		 FROM events WHERE tenant_id=$1 AND dedup_key=$2`,
		tenantID, dedupKey,
	)
	var e domain.Event
	err := row.Scan(&e.ID, &e.TenantID, &e.EventType, &e.Payload, &e.DedupKey, &e.Status, &e.CreatedAt, &e.CreatedBy)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg: find event by dedup: %w", err)
	}
	return &e, nil
}

func (r *PGEventRepo) FindByID(ctx context.Context, id string) (*domain.Event, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, event_type, payload, dedup_key, status, created_at, created_by
		 FROM events WHERE id=$1`, id,
	)
	var e domain.Event
	err := row.Scan(&e.ID, &e.TenantID, &e.EventType, &e.Payload, &e.DedupKey, &e.Status, &e.CreatedAt, &e.CreatedBy)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg: find event by id: %w", err)
	}
	return &e, nil
}

// PGSubscriptionRepo implements repository.SubscriptionRepository.
type PGSubscriptionRepo struct{ pool *pgxpool.Pool }

func NewPGSubscriptionRepo(pool *pgxpool.Pool) *PGSubscriptionRepo {
	return &PGSubscriptionRepo{pool: pool}
}
func (r *PGSubscriptionRepo) Create(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO subscriptions (tenant_id, event_type, target_url, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		sub.TenantID, sub.EventType, sub.TargetURL, sub.CreatedBy,
	)
	if err := row.Scan(&sub.ID, &sub.CreatedAt); err != nil {
		return nil, fmt.Errorf("pg: insert subscription: %w", err)
	}
	return sub, nil
}
func (r *PGSubscriptionRepo) FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, event_type, target_url, created_at, created_by
		 FROM subscriptions WHERE id=$1 AND tenant_id=$2`, id, tenantID,
	)
	var s domain.Subscription
	err := row.Scan(&s.ID, &s.TenantID, &s.EventType, &s.TargetURL, &s.CreatedAt, &s.CreatedBy)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg: find subscription: %w", err)
	}
	return &s, nil
}
func (r *PGSubscriptionRepo) ListByTenant(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, event_type, target_url, created_at, created_by
		 FROM subscriptions WHERE tenant_id=$1 ORDER BY created_at`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("pg: list subscriptions: %w", err)
	}
	defer rows.Close()
	var out []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		if err := rows.Scan(&s.ID, &s.TenantID, &s.EventType, &s.TargetURL, &s.CreatedAt, &s.CreatedBy); err != nil {
			return nil, fmt.Errorf("pg: scan subscription: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *PGSubscriptionRepo) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM subscriptions WHERE id=$1 AND tenant_id=$2`, id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("pg: delete subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// PGOutboxRepo implements OutboxRepository.
type PGOutboxRepo struct{ pool *pgxpool.Pool }

func NewPGOutboxRepo(pool *pgxpool.Pool) *PGOutboxRepo { return &PGOutboxRepo{pool: pool} }
func (r *PGOutboxRepo) Create(ctx context.Context, entry *domain.OutboxEntry) error {
	var q interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	} = r.pool
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		q = tx
	}
	row := q.QueryRow(ctx,
		`INSERT INTO outbox (event_id, subscription_id, status, next_attempt_at)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`,
		entry.EventID, entry.SubscriptionID, entry.Status, entry.NextAttemptAt,
	)
	return row.Scan(&entry.ID, &entry.CreatedAt, &entry.UpdatedAt)
}

func (r *PGOutboxRepo) ClaimPending(ctx context.Context, now time.Time, limit int) ([]domain.OutboxEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, event_id, subscription_id, status, attempts, next_attempt_at, last_error, created_at, updated_at
		 FROM outbox
		 WHERE status='pending' AND next_attempt_at <= $1
		 ORDER BY next_attempt_at
		 LIMIT $2
		 FOR UPDATE SKIP LOCKED`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("pg: claim pending: %w", err)
	}
	defer rows.Close()
	var out []domain.OutboxEntry
	for rows.Next() {
		var e domain.OutboxEntry
		if err := rows.Scan(&e.ID, &e.EventID, &e.SubscriptionID, &e.Status, &e.Attempts,
			&e.NextAttemptAt, &e.LastError, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pg: scan outbox: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (r *PGOutboxRepo) MarkDelivered(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbox SET status='delivered', updated_at=now() WHERE id=$1`, id,
	)
	return err
}
func (r *PGOutboxRepo) MarkFailed(ctx context.Context, id string, attempts int, nextAttemptAt time.Time, lastError string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbox SET status='failed', attempts=$2, next_attempt_at=$3, last_error=$4, updated_at=now()
		 WHERE id=$1`, id, attempts, nextAttemptAt, lastError,
	)
	return err
}
func (r *PGOutboxRepo) MarkDeadLetter(ctx context.Context, id, lastError string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE outbox SET status='dead_letter', last_error=$2, updated_at=now() WHERE id=$1`, id, lastError,
	)
	return err
}

// PGTxManager implements TxManager using pgx.
type PGTxManager struct{ pool *pgxpool.Pool }

func NewPGTxManager(pool *pgxpool.Pool) *PGTxManager { return &PGTxManager{pool: pool} }

func (m *PGTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pg: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	ctxWithTx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(ctxWithTx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pg: commit tx: %w", err)
	}
	return nil
}
