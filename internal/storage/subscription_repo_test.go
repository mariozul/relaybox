package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariozul/relaybox/internal/domain"
)

func testPoolSub(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testPool(t)

	ctx := context.Background()
	pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS subscriptions (
		    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    tenant_id TEXT NOT NULL,
		    event_type TEXT NOT NULL,
		    target_url TEXT NOT NULL,
		    created_by TEXT NOT NULL DEFAULT 'system',
		    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		    updated_by TEXT NOT NULL DEFAULT 'system',
		    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS subscriptions CASCADE")
	})

	return pool
}

func TestPostgresSubscriptionRepo_CRUD(t *testing.T) {
	t.Parallel()

	pool := testPoolSub(t)
	repo := NewPostgresSubscriptionRepository(pool)
	ctx := context.Background()
	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)

	sub := domain.NewSubscription("tenant-a", "order.created", "https://example.com/hook", "system", now)

	// Create.
	created, err := repo.Create(ctx, sub)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// GetByID.
	got, err := repo.GetByID(ctx, "tenant-a", created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("ID mismatch")
	}

	// Cross-tenant get returns not found.
	_, err = repo.GetByID(ctx, "tenant-b", created.ID)
	if !domain.IsNotFound(err) {
		t.Fatalf("expected ErrNotFound for cross-tenant, got %v", err)
	}

	// ListByTenant.
	list, err := repo.ListByTenant(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(list))
	}

	// Empty list for other tenant.
	listB, err := repo.ListByTenant(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("list tenant-b: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected 0 subs for tenant-b, got %d", len(listB))
	}

	// ListByEventType.
	listE, err := repo.ListByEventType(ctx, "tenant-a", "order.created")
	if err != nil {
		t.Fatalf("list by type: %v", err)
	}
	if len(listE) != 1 {
		t.Fatalf("expected 1 sub matching event type, got %d", len(listE))
	}

	// Delete cross-tenant fails.
	err = repo.Delete(ctx, "tenant-b", created.ID)
	if !domain.IsNotFound(err) {
		t.Fatalf("expected ErrNotFound for cross-tenant delete, got %v", err)
	}

	// Delete.
	if err := repo.Delete(ctx, "tenant-a", created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Get after delete.
	_, err = repo.GetByID(ctx, "tenant-a", created.ID)
	if !domain.IsNotFound(err) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
