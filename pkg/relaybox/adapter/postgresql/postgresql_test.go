// Package postgresql_test contains integration tests for the PostgreSQL adapter
// implementations of EventRepository, SubscriptionRepository, and OutboxRepository.
// These tests require a live PostgreSQL database. If the database is unreachable,
// tests are gracefully skipped (TC-08-EDGE pattern: readyz 503).
package postgresql_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/mariozul/relaybox/pkg/relaybox/adapter/postgresql"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// testDB is the global test database connection, initialized once per package.
// It is nil when the database is unreachable, causing tests to skip.
var testDB *sql.DB

// TestMain initializes the test database connection. If the environment variable
// RELAYBOX_TEST_DSN is not set or the connection fails, tests skip gracefully
// instead of failing. This enables CI environments without PostgreSQL to pass.
func TestMain(m *testing.M) {
	dsn := os.Getenv("RELAYBOX_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/relaybox_test?sslmode=disable"
	}

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		// Connection string parsing failed; skip all tests.
		os.Exit(m.Run())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := testDB.PingContext(ctx); err != nil {
		// Database unreachable; skip all tests.
		testDB = nil
		os.Exit(m.Run())
	}

	// Run the migrations to set up test tables.
	if err := runMigrations(testDB); err != nil {
		panic("failed to run migrations: " + err.Error())
	}

	// Run tests.
	code := m.Run()

	// Cleanup: drop test tables.
	tearDown(testDB)
	os.Exit(code)
}

func runMigrations(db *sql.DB) error {
	migration := `
	BEGIN;

	CREATE TABLE IF NOT EXISTS events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		payload JSONB NOT NULL DEFAULT '{}',
		dedup_key TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by TEXT NOT NULL DEFAULT '',
		updated_by TEXT NOT NULL DEFAULT '',
		CONSTRAINT uq_events_tenant_dedup UNIQUE (tenant_id, dedup_key)
	);

	CREATE INDEX IF NOT EXISTS idx_events_tenant_id ON events (tenant_id);
	CREATE INDEX IF NOT EXISTS idx_events_event_type ON events (event_type);

	CREATE TABLE IF NOT EXISTS subscriptions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		target_url TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by TEXT NOT NULL DEFAULT '',
		updated_by TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_id ON subscriptions (tenant_id);
	CREATE INDEX IF NOT EXISTS idx_subscriptions_event_type ON subscriptions (event_type);

	CREATE TABLE IF NOT EXISTS outbox (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
		status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivered', 'failed', 'deadletter')),
		attempts INT NOT NULL DEFAULT 0,
		next_attempt_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT uq_outbox_event_subscription UNIQUE (event_id, subscription_id)
	);

	CREATE INDEX IF NOT EXISTS idx_outbox_pending
		ON outbox (status, next_attempt_at NULLS FIRST)
		WHERE status IN ('pending', 'failed');

	CREATE INDEX IF NOT EXISTS idx_outbox_event_id ON outbox (event_id);

	COMMIT;
	`
	_, err := db.ExecContext(context.Background(), migration)
	return err
}

func tearDown(db *sql.DB) {
	_, _ = db.ExecContext(context.Background(), `
		DROP TABLE IF EXISTS outbox CASCADE;
		DROP TABLE IF EXISTS subscriptions CASCADE;
		DROP TABLE IF EXISTS events CASCADE;
	`)
}

// skipIfNoDB skips the current test if the database connection is unavailable.
func skipIfNoDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("postgreSQL not available: set RELAYBOX_TEST_DSN or ensure local postgres is running")
	}
}

// ============================================================================
// TC-01-UNIT: Successful event ingestion and outbox entry creation (FR-ING-01, FR-ING-02)
// ============================================================================

func TestEventRepository_Save_Success(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	// Create repositories using the same database.
	eventRepo := postgresql.NewEventRepository(testDB)
	subRepo := postgresql.NewSubscriptionRepository(testDB)
	outboxRepo := postgresql.NewOutboxRepository(testDB)

	// Create a subscription first, because outbox FK references subscriptions.
	sub, err := domain.NewSubscription("tenant-save-test", "order.created",
		"https://example.com/webhook", "tester", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	savedSub, err := subRepo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	// Create an event.
	event, err := domain.NewEvent("tenant-save-test", "order.created",
		[]byte(`{"order_id":"abc123"}`), "dedup-save-001", "tester", now)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	// Save the event (transactional: event + outbox).
	savedEvent, err := eventRepo.SaveWithOutbox(ctx, event, savedSub.ID, now)
	if err != nil {
		t.Fatalf("SaveWithOutbox() returned error: %v", err)
	}

	if savedEvent == nil {
		t.Fatal("SaveWithOutbox() returned nil event")
	}
	if savedEvent.ID == "" {
		t.Error("expected event ID to be set")
	}
	if savedEvent.TenantID != "tenant-save-test" {
		t.Errorf("expected TenantID=%q, got %q", "tenant-save-test", savedEvent.TenantID)
	}

	// Verify the outbox entry was created.
	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}

	var found bool
	for _, entry := range entries {
		if entry.EventID == savedEvent.ID && entry.SubscriptionID == savedSub.ID {
			found = true
			if entry.Status != domain.OutboxStatusPending {
				t.Errorf("expected outbox status=pending, got %q", entry.Status)
			}
			break
		}
	}
	if !found {
		t.Error("expected outbox entry to be created for the saved event")
	}
}

// ============================================================================
// TC-02-EDGE: Idempotent ingestion of duplicate event (FR-ING-03, AC-01)
// ============================================================================

func TestEventRepository_DuplicateDedupKey(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	eventRepo := postgresql.NewEventRepository(testDB)
	subRepo := postgresql.NewSubscriptionRepository(testDB)

	sub, err := domain.NewSubscription("tenant-dedup-test", "order.created",
		"https://example.com/webhook", "tester", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	savedSub, err := subRepo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	// First ingestion.
	dedupKey := "dedup-unique-002"
	event1, err := domain.NewEvent("tenant-dedup-test", "order.created",
		[]byte(`{"order_id":"first"}`), dedupKey, "tester", now)
	if err != nil {
		t.Fatalf("failed to create event1: %v", err)
	}
	saved1, err := eventRepo.SaveWithOutbox(ctx, event1, savedSub.ID, now)
	if err != nil {
		t.Fatalf("first SaveWithOutbox() returned error: %v", err)
	}

	// Second ingestion with same dedup_key - should return ErrDuplicateEvent.
	event2, err := domain.NewEvent("tenant-dedup-test", "order.created",
		[]byte(`{"order_id":"second"}`), dedupKey, "tester", now)
	if err != nil {
		t.Fatalf("failed to create event2: %v", err)
	}

	_, err = eventRepo.SaveWithOutbox(ctx, event2, savedSub.ID, now)
	if err == nil {
		t.Fatal("expected ErrDuplicateEvent for duplicate dedup_key, got nil")
	}
	if !errors.Is(err, domain.ErrDuplicateEvent) {
		t.Errorf("expected ErrDuplicateEvent, got %v", err)
	}

	// Verify the original event is still there via FindByDedupKey.
	found, err := eventRepo.FindByDedupKey(ctx, "tenant-dedup-test", dedupKey)
	if err != nil {
		t.Fatalf("FindByDedupKey() returned error: %v", err)
	}
	if found.ID != saved1.ID {
		t.Errorf("expected original event ID=%q, got %q", saved1.ID, found.ID)
	}
}

// ============================================================================
// TC-04-UNIT: CRUD operations for subscriptions (FR-SUB-01)
// ============================================================================

func TestSubscriptionRepository_SaveAndFind(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	repo := postgresql.NewSubscriptionRepository(testDB)

	// Create subscription.
	sub, err := domain.NewSubscription("tenant-crud-test", "user.updated",
		"https://example.com/user-updates", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	saved, err := repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if saved.ID == "" {
		t.Error("expected saved subscription to have an ID")
	}
	if saved.TargetURL != "https://example.com/user-updates" {
		t.Errorf("expected TargetURL=%q, got %q", "https://example.com/user-updates", saved.TargetURL)
	}

	// Find by ID.
	found, err := repo.FindByID(ctx, "tenant-crud-test", saved.ID)
	if err != nil {
		t.Fatalf("FindByID() returned error: %v", err)
	}
	if found.ID != saved.ID {
		t.Errorf("expected ID=%q, got %q", saved.ID, found.ID)
	}
	if found.EventType != "user.updated" {
		t.Errorf("expected EventType=%q, got %q", "user.updated", found.EventType)
	}
}

func TestSubscriptionRepository_Delete(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	repo := postgresql.NewSubscriptionRepository(testDB)

	sub, err := domain.NewSubscription("tenant-delete-test", "order.cancelled",
		"https://example.com/cancellations", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	saved, err := repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Delete.
	err = repo.Delete(ctx, "tenant-delete-test", saved.ID)
	if err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	// Verify deletion.
	_, err = repo.FindByID(ctx, "tenant-delete-test", saved.ID)
	if err == nil {
		t.Fatal("expected ErrSubscriptionNotFound after deletion, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionRepository_ListByTenant(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	repo := postgresql.NewSubscriptionRepository(testDB)

	// Create multiple subscriptions for the same tenant.
	sub1, _ := domain.NewSubscription("tenant-list-test", "order.created",
		"https://example.com/orders", "admin", now)
	sub2, _ := domain.NewSubscription("tenant-list-test", "user.updated",
		"https://example.com/users", "admin", now)
	sub3, _ := domain.NewSubscription("other-tenant", "order.created",
		"https://other.com/hooks", "admin", now)

	_, _ = repo.Save(ctx, sub1)
	_, _ = repo.Save(ctx, sub2)
	_, _ = repo.Save(ctx, sub3)

	subs, err := repo.FindByTenantID(ctx, "tenant-list-test")
	if err != nil {
		t.Fatalf("FindByTenantID() returned error: %v", err)
	}

	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions for tenant-list-test, got %d", len(subs))
	}
}

func TestSubscriptionRepository_FindByID_NotFound(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	repo := postgresql.NewSubscriptionRepository(testDB)

	_, err := repo.FindByID(ctx, "tenant-xyz", "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for nonexistent subscription, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// ============================================================================
// TC-05-INT: Tenant isolation for subscriptions (FR-SUB-02, AC-04, RULE-SEC-01)
// ============================================================================

func TestSubscriptionRepository_TenantIsolation_FindByID(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	repo := postgresql.NewSubscriptionRepository(testDB)

	// Tenant A creates a subscription.
	sub, err := domain.NewSubscription("tenant-a", "order.created",
		"https://tenant-a.example.com/webhook", "admin-a", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	saved, err := repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Tenant B should NOT be able to access Tenant A's subscription.
	_, err = repo.FindByID(ctx, "tenant-b", saved.ID)
	if err == nil {
		t.Fatal("expected ErrSubscriptionNotFound for cross-tenant access, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionRepository_TenantIsolation_Delete(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	repo := postgresql.NewSubscriptionRepository(testDB)

	// Tenant A creates a subscription.
	sub, err := domain.NewSubscription("tenant-a-delete", "order.created",
		"https://tenant-a.example.com/webhook", "admin-a", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	saved, err := repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Tenant B should NOT be able to delete Tenant A's subscription.
	err = repo.Delete(ctx, "tenant-b", saved.ID)
	if err == nil {
		t.Fatal("expected ErrSubscriptionNotFound for cross-tenant delete, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}

	// Verify Tenant A's subscription still exists.
	_, err = repo.FindByID(ctx, "tenant-a-delete", saved.ID)
	if err != nil {
		t.Errorf("original subscription should still exist, got error: %v", err)
	}
}

// ============================================================================
// TC-08-EDGE: Outbox CRUD operations
// ============================================================================

func TestOutboxRepository_CRUD(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	eventRepo := postgresql.NewEventRepository(testDB)
	subRepo := postgresql.NewSubscriptionRepository(testDB)
	outboxRepo := postgresql.NewOutboxRepository(testDB)

	// Create prerequisite: event + subscription.
	sub, _ := domain.NewSubscription("tenant-outbox-test", "order.created",
		"https://example.com/webhook", "tester", now)
	savedSub, _ := subRepo.Save(ctx, sub)

	event, _ := domain.NewEvent("tenant-outbox-test", "order.created",
		[]byte(`{"id":"outbox-test"}`), "dedup-outbox-001", "tester", now)
	savedEvent, err := eventRepo.SaveWithOutbox(ctx, event, savedSub.ID, now)
	if err != nil {
		t.Fatalf("SaveWithOutbox() returned error: %v", err)
	}

	// Find pending entries - should include the newly created one.
	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}

	var targetEntry *domain.OutboxEntry
	for i, entry := range entries {
		if entry.EventID == savedEvent.ID {
			targetEntry = &entries[i]
			break
		}
	}
	if targetEntry == nil {
		t.Fatal("expected outbox entry to exist for the saved event")
	}

	// Update status to delivered.
	err = outboxRepo.UpdateStatus(ctx, targetEntry.ID, domain.OutboxStatusDelivered)
	if err != nil {
		t.Fatalf("UpdateStatus() returned error: %v", err)
	}

	// Verify it's no longer pending.
	entriesAfter, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}
	for _, entry := range entriesAfter {
		if entry.ID == targetEntry.ID {
			t.Errorf("expected delivered entry to not appear in FindPending")
		}
	}
}

func TestOutboxRepository_FindPending_Empty(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	repo := postgresql.NewOutboxRepository(testDB)

	// When no pending entries exist, should return empty slice, not nil.
	entries, err := repo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}
	if entries == nil {
		t.Error("expected empty slice, not nil")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 pending entries, got %d", len(entries))
	}
}

func TestOutboxRepository_UpdateStatus_NotFound(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	repo := postgresql.NewOutboxRepository(testDB)

	err := repo.UpdateStatus(ctx, "00000000-0000-0000-0000-000000000000", domain.OutboxStatusDelivered)
	if err == nil {
		t.Fatal("expected ErrNotFound for nonexistent outbox entry, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ============================================================================
// Outbox transactional atomicity tests (FR-ING-02, RULE-DATA-01)
// ============================================================================

func TestEventRepository_TransactionalAtomicity(t *testing.T) {
	skipIfNoDB(t)
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()

	eventRepo := postgresql.NewEventRepository(testDB)
	subRepo := postgresql.NewSubscriptionRepository(testDB)
	outboxRepo := postgresql.NewOutboxRepository(testDB)

	// Create a subscription.
	sub, _ := domain.NewSubscription("tenant-tx-test", "order.created",
		"https://example.com/webhook", "tester", now)
	savedSub, _ := subRepo.Save(ctx, sub)

	// Save event with outbox transactionally.
	event, _ := domain.NewEvent("tenant-tx-test", "order.created",
		[]byte(`{"order_id":"tx-test"}`), "dedup-tx-001", "tester", now)

	savedEvent, err := eventRepo.SaveWithOutbox(ctx, event, savedSub.ID, now)
	if err != nil {
		t.Fatalf("SaveWithOutbox() returned error: %v", err)
	}

	// Verify that both the event AND the outbox entry exist.
	found, err := eventRepo.FindByDedupKey(ctx, "tenant-tx-test", "dedup-tx-001")
	if err != nil {
		t.Fatalf("FindByDedupKey() returned error: %v", err)
	}
	if found == nil {
		t.Fatal("expected event to exist after SaveWithOutbox")
	}

	entries, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}
	var hasOutbox bool
	for _, e := range entries {
		if e.EventID == savedEvent.ID {
			hasOutbox = true
			break
		}
	}
	if !hasOutbox {
		t.Error("expected outbox entry to exist for the saved event (transactional atomicity)")
	}
}

// ============================================================================
// Ready/Health check pattern: test that DB unreachability is detected
// (TC-08-EDGE partial — full health check in ST-09)
// ============================================================================

func TestDBChecker_Unreachable(t *testing.T) {
	// This test verifies that when the DB is unreachable (simulated by an
	// intentionally bad DSN), the checker correctly returns an error.
	// This is the TC-08-EDGE pattern for /readyz returning 503.

	badDB, err := sql.Open("postgres", "postgres://localhost:19999/doesnotexist?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("unexpected error opening bad DSN: %v", err)
	}
	defer badDB.Close()

	checker := postgresql.NewDBChecker(badDB)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = checker.CheckDB(ctx)
	if err == nil {
		t.Error("expected error for unreachable DB, got nil")
	}
}
