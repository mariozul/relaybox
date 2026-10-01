package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/domain"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://relaybox:relaybox@localhost:5432/relaybox?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to postgres: %v", err)
	}

	// Run migrations manually via SQL.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS events (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    tenant_id TEXT NOT NULL,
		    event_type TEXT NOT NULL,
		    payload BYTEA NOT NULL,
		    dedup_key TEXT NOT NULL,
		    created_by TEXT NOT NULL DEFAULT 'system',
		    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		    updated_by TEXT NOT NULL DEFAULT 'system',
		    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		    CONSTRAINT uq_events_test UNIQUE (tenant_id, dedup_key)
		)
	`); err != nil {
		pool.Close()
		t.Skipf("skipping: migration error: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS events CASCADE")
		pool.Close()
	})

	return pool
}

func TestPostgresEventRepo_CreateOrGet(t *testing.T) {
	t.Parallel()

	pool := testPool(t)
	repo := NewPostgresEventRepository(pool)
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)

	event := domain.NewEvent("tenant-a", "order.created", []byte(`{"x":1}`), "dk-1", "system", now)

	// First insert.
	result1, err := repo.CreateOrGet(ctx, event)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if result1.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if result1.TenantID != "tenant-a" {
		t.Fatalf("tenant mismatch: %s", result1.TenantID)
	}

	// Second insert with same dedup key returns the existing event.
	result2, err := repo.CreateOrGet(ctx, event)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if result2.ID != result1.ID {
		t.Fatalf("idempotent insert returned different ID: %s vs %s", result2.ID, result1.ID)
	}
}
