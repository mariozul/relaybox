package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// setupTestDB opens a test database connection and applies schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	host := getEnvOrDefault("TEST_DB_HOST", "localhost")
	port := getEnvOrDefault("TEST_DB_PORT", "5432")
	user := getEnvOrDefault("TEST_DB_USER", "relaybox")
	password := getEnvOrDefault("TEST_DB_PASSWORD", "relaybox")
	dbname := getEnvOrDefault("TEST_DB_NAME", "relaybox_test")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("test DB not reachable (dsn: host=%s port=%s user=%s dbname=%s): %v", host, port, user, dbname, err)
	}

	applyMigrations(t, db, ctx)
	return db
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func applyMigrations(t *testing.T, db *sql.DB, ctx context.Context) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS events (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			event_type VARCHAR(255) NOT NULL,
			payload JSONB NOT NULL,
			dedup_key VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			created_by VARCHAR(255) NOT NULL,
			UNIQUE (tenant_id, dedup_key)
		)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id VARCHAR(36) NOT NULL,
			event_type VARCHAR(255) NOT NULL,
			target_url TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			created_by VARCHAR(255) NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_by VARCHAR(255) NOT NULL,
			UNIQUE (tenant_id, event_type, target_url)
		)`,
		`CREATE TABLE IF NOT EXISTS outbox (
			id VARCHAR(36) PRIMARY KEY,
			event_id VARCHAR(36) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			subscription_id VARCHAR(36) NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			last_attempt_at TIMESTAMP WITH TIME ZONE,
			last_attempt_error TEXT,
			delivery_id VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			UNIQUE (delivery_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_status_next_attempt_at ON outbox (status, next_attempt_at)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("migration failed: %v\nstmt: %s", err, s)
		}
	}
	t.Cleanup(func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(cleanCtx, "DELETE FROM outbox")
		_, _ = db.ExecContext(cleanCtx, "DELETE FROM events")
		_, _ = db.ExecContext(cleanCtx, "DELETE FROM subscriptions")
	})
}

func makeEvent(tenantID string) *domain.Event {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Event{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"ord-123"}`),
		DedupKey:  uuid.New().String(),
		CreatedAt: now,
		CreatedBy: "test-user",
	}
}

func makeSubscription(tenantID string) *domain.Subscription {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Subscription{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
		CreatedAt: now,
		CreatedBy: "test-user",
		UpdatedAt: now,
		UpdatedBy: "test-user",
	}
}
