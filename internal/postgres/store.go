package postgres

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/tenant"
)

//go:embed migrations/001_init.sql
var schema string

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if _, err = pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) Ingest(ctx context.Context, event domain.Event) (domain.Event, bool, error) {
	tenantID, err := tenant.Require(ctx)
	if err != nil {
		return domain.Event{}, false, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Event{}, false, fmt.Errorf("begin ingest: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	event.ID, event.TenantID, event.CreatedAt = newID(), tenantID, time.Now().UTC()
	row := tx.QueryRow(ctx, `INSERT INTO events(id,tenant_id,event_type,payload,dedup_key,created_at,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id,dedup_key) DO NOTHING RETURNING id`, event.ID, tenantID, event.EventType, event.Payload, event.DedupKey, event.CreatedAt, event.CreatedBy)
	created := true
	if err = row.Scan(&event.ID); errors.Is(err, pgx.ErrNoRows) {
		created = false
		err = tx.QueryRow(ctx, `SELECT id,created_at,created_by FROM events WHERE tenant_id=$1 AND dedup_key=$2`, tenantID, event.DedupKey).Scan(&event.ID, &event.CreatedAt, &event.CreatedBy)
	}
	if err != nil {
		return domain.Event{}, false, fmt.Errorf("insert event: %w", err)
	}
	if created {
		_, err = tx.Exec(ctx, `INSERT INTO outbox(id,tenant_id,event_id,subscription_id,next_attempt_at)
			SELECT gen_random_uuid(),tenant_id,$1,id,$2 FROM subscriptions WHERE tenant_id=$3 AND event_type=$4`, event.ID, event.CreatedAt, tenantID, event.EventType)
		if err != nil {
			return domain.Event{}, false, fmt.Errorf("enqueue event: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Event{}, false, fmt.Errorf("commit ingest: %w", err)
	}
	return event, created, nil
}

func (s *Store) CreateSubscription(ctx context.Context, sub domain.Subscription) (domain.Subscription, error) {
	tenantID, err := tenant.Require(ctx)
	if err != nil {
		return domain.Subscription{}, err
	}
	sub.ID, sub.TenantID, sub.CreatedAt = newID(), tenantID, time.Now().UTC()
	_, err = s.pool.Exec(ctx, `INSERT INTO subscriptions(id,tenant_id,event_type,target_url,created_at) VALUES($1,$2,$3,$4,$5)`, sub.ID, tenantID, sub.EventType, sub.TargetURL, sub.CreatedAt)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("create subscription: %w", err)
	}
	return sub, nil
}

func (s *Store) ListSubscriptions(ctx context.Context) ([]domain.Subscription, error) {
	tenantID, err := tenant.Require(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,tenant_id,event_type,target_url,created_at FROM subscriptions WHERE tenant_id=$1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Subscription])
	if err != nil {
		return nil, fmt.Errorf("scan subscriptions: %w", err)
	}
	return items, nil
}

func (s *Store) DeleteSubscription(ctx context.Context, id string) error {
	tenantID, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM subscriptions WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}

func (s *Store) Claim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]domain.Delivery, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT o.id,o.tenant_id,o.event_id,o.subscription_id,e.event_type,e.payload,s.target_url,o.attempts
		FROM outbox o JOIN events e ON e.id=o.event_id JOIN subscriptions s ON s.id=o.subscription_id
		WHERE o.next_attempt_at<=$1 AND (o.status='pending' OR (o.status='inflight' AND o.lease_until<=$1))
		ORDER BY o.next_attempt_at FOR UPDATE OF o SKIP LOCKED LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("select outbox: %w", err)
	}
	defer rows.Close()
	var deliveries []domain.Delivery
	for rows.Next() {
		var d domain.Delivery
		if err = rows.Scan(&d.ID, &d.TenantID, &d.EventID, &d.SubscriptionID, &d.EventType, &d.Payload, &d.TargetURL, &d.Attempts); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		d.LeaseToken = newID()
		if _, err = tx.Exec(ctx, `UPDATE outbox SET status='inflight',lease_token=$1,lease_until=$2 WHERE id=$3`, d.LeaseToken, now.Add(lease), d.ID); err != nil {
			return nil, fmt.Errorf("lease outbox: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim: %w", err)
	}
	return deliveries, nil
}

func (s *Store) MarkDelivered(ctx context.Context, d domain.Delivery, at time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET status='delivered',attempts=attempts+1,delivered_at=$1,lease_token=NULL,lease_until=NULL WHERE id=$2 AND lease_token=$3`, at, d.ID, d.LeaseToken)
	if err != nil {
		return fmt.Errorf("mark delivered: %w", err)
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, d domain.Delivery, next time.Time, reason string, dead bool) error {
	status := "pending"
	if dead {
		status = "deadletter"
	}
	_, err := s.pool.Exec(ctx, `UPDATE outbox SET status=$1,attempts=attempts+1,next_attempt_at=$2,last_error=$3,lease_token=NULL,lease_until=NULL WHERE id=$4 AND lease_token=$5`, status, next, reason, d.ID, d.LeaseToken)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}
