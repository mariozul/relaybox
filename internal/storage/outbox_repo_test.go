package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/domain"
)

func testPoolOutbox(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testPool(t)

	ctx := context.Background()
	pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS subscriptions (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    tenant_id TEXT NOT NULL,
		    event_type TEXT NOT NULL,
		    target_url TEXT NOT NULL,
		    created_by TEXT NOT NULL,
		    created_at TIMESTAMPTZ NOT NULL,
		    updated_by TEXT NOT NULL,
		    updated_at TIMESTAMPTZ NOT NULL
		)
	`)
	pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS events (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    tenant_id TEXT NOT NULL,
		    event_type TEXT NOT NULL,
		    payload BYTEA NOT NULL,
		    dedup_key TEXT NOT NULL,
		    created_by TEXT NOT NULL,
		    created_at TIMESTAMPTZ NOT NULL,
		    updated_by TEXT NOT NULL,
		    updated_at TIMESTAMPTZ NOT NULL
		)
	`)
	pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS outbox (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    event_id UUID NOT NULL REFERENCES events(id),
		    subscription_id UUID NOT NULL REFERENCES subscriptions(id),
		    status TEXT NOT NULL DEFAULT 'pending',
		    attempts INT NOT NULL DEFAULT 0,
		    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		    delivery_id UUID NOT NULL DEFAULT gen_random_uuid(),
		    created_by TEXT NOT NULL,
		    created_at TIMESTAMPTZ NOT NULL,
		    updated_by TEXT NOT NULL,
		    updated_at TIMESTAMPTZ NOT NULL
		)
	`)

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS outbox CASCADE")
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS events CASCADE")
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS subscriptions CASCADE")
	})

	return pool
}

func testFixtures(t *testing.T, pool *pgxpool.Pool) (eventID, subID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)

	// Insert event.
	row := pool.QueryRow(ctx, `
		INSERT INTO events (tenant_id, event_type, payload, dedup_key, created_by, created_at, updated_by, updated_at)
		VALUES ('tenant-a', 'order.created', $1, 'dk-1', 'system', $2, 'system', $2)
		RETURNING id
	`, []byte(`{"x":1}`), now)
	if err := row.Scan(&eventID); err != nil {
		t.Fatalf("fixture event: %v", err)
	}

	// Insert subscription.
	row2 := pool.QueryRow(ctx, `
		INSERT INTO subscriptions (tenant_id, event_type, target_url, created_by, created_at, updated_by, updated_at)
		VALUES ('tenant-a', 'order.created', 'https://example.com/hook', 'system', $1, 'system', $1)
		RETURNING id
	`, now)
	if err := row2.Scan(&subID); err != nil {
		t.Fatalf("fixture sub: %v", err)
	}

	return eventID, subID
}

func TestPostgresOutboxRepo_BatchCreateAndClaim(t *testing.T) {
	t.Parallel()

	pool := testPoolOutbox(t)
	repo := NewPostgresOutboxRepository(pool)
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)
	eventID, subID := testFixtures(t, pool)

	// Create batch.
	msg := domain.NewOutboxMessage(eventID, subID, "del-1", "system", now)
	msg.DeliveryID = "del-1"
	err := repo.CreateBatch(ctx, []domain.OutboxMessage{msg})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	// Claim pending.
	claimed, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim pending: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected 1 claimed, got %d", len(claimed))
	}
	if claimed[0].DeliveryID != "del-1" {
		t.Fatalf("delivery_id mismatch: %s", claimed[0].DeliveryID)
	}
}

func TestPostgresOutboxRepo_MarkDelivered(t *testing.T) {
	t.Parallel()

	pool := testPoolOutbox(t)
	repo := NewPostgresOutboxRepository(pool)
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)
	eventID, subID := testFixtures(t, pool)

	msg := domain.NewOutboxMessage(eventID, subID, "del-1", "system", now)
	msg.DeliveryID = "del-1"
	if err := repo.CreateBatch(ctx, []domain.OutboxMessage{msg}); err != nil {
		t.Fatalf("create batch: %v", err)
	}

	claimed, _ := repo.ClaimPending(ctx, 10)
	if err := repo.MarkDelivered(ctx, claimed[0].ID); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}

	// Should not be claimed again.
	claimed2, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim after deliver: %v", err)
	}
	if len(claimed2) != 0 {
		t.Fatalf("expected 0 pending after delivery, got %d", len(claimed2))
	}
}

func TestPostgresOutboxRepo_MarkRetryAndDeadLetter(t *testing.T) {
	t.Parallel()

	pool := testPoolOutbox(t)
	repo := NewPostgresOutboxRepository(pool)
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)
	eventID, subID := testFixtures(t, pool)

	msg := domain.NewOutboxMessage(eventID, subID, "del-1", "system", now)
	msg.DeliveryID = "del-1"
	if err := repo.CreateBatch(ctx, []domain.OutboxMessage{msg}); err != nil {
		t.Fatalf("create batch: %v", err)
	}

	claimed, _ := repo.ClaimPending(ctx, 10)
	retryTime := now.Add(30 * time.Second)
	if err := repo.MarkRetry(ctx, claimed[0].ID, retryTime); err != nil {
		t.Fatalf("mark retry: %v", err)
	}

	// Not claimable yet because next_attempt_at is in future.
	claimed2, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim after retry: %v", err)
	}
	if len(claimed2) != 0 {
		t.Fatalf("expected 0 pending with future retry, got %d", len(claimed2))
	}

	// Dead letter.
	if err := repo.DeadLetter(ctx, claimed[0].ID); err != nil {
		t.Fatalf("dead letter: %v", err)
	}

	// Dead-lettered row is not claimable.
	claimed3, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim after deadletter: %v", err)
	}
	if len(claimed3) != 0 {
		t.Fatalf("expected 0 pending after deadletter, got %d", len(claimed3))
	}
}
