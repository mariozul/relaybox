package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	repoimpl "github.com/mariozul/relaybox/pkg/relaybox/infrastructure/repository"
)

// ============================================================
// Outbox Repository Tests
// ============================================================
func TestPostgresOutboxRepository_EnqueueEvent(t *testing.T) {
	db := setupTestDB(t)
	eventRepo := repoimpl.NewPostgresEventRepository(db)
	outboxRepo := repoimpl.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")
	_, err := eventRepo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	sub := makeSubscription("tenant-alpha")
	sub.EventType = event.EventType
	_, err = db.ExecContext(ctx,
		`INSERT INTO subscriptions (id, tenant_id, event_type, target_url, created_at, created_by, updated_at, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		sub.ID, sub.TenantID, sub.EventType, sub.TargetURL,
		sub.CreatedAt, sub.CreatedBy, sub.UpdatedAt, sub.UpdatedBy)
	if err != nil {
		t.Fatalf("insert sub: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	entry := &domain.OutboxEntry{
		ID:             uuid.New().String(),
		EventID:        event.ID,
		SubscriptionID: sub.ID,
		Status:         "pending",
		Attempts:       0,
		NextAttemptAt:  now,
		DeliveryID:     uuid.New().String(),
		CreatedAt:      now,
	}

	id, err := outboxRepo.EnqueueEvent(ctx, entry)
	if err != nil {
		t.Fatalf("EnqueueEvent: %v", err)
	}
	if id != entry.ID {
		t.Errorf("id: got %s, want %s", id, entry.ID)
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM outbox WHERE id=$1", entry.ID).Scan(&count)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 outbox row, got %d", count)
	}
}

func TestPostgresOutboxRepository_GetPendingEntries(t *testing.T) {
	db := setupTestDB(t)
	eventRepo := repoimpl.NewPostgresEventRepository(db)
	outboxRepo := repoimpl.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")
	_, err := eventRepo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	sub := makeSubscription("tenant-alpha")
	_, err = db.ExecContext(ctx,
		`INSERT INTO subscriptions (id, tenant_id, event_type, target_url, created_at, created_by, updated_at, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		sub.ID, sub.TenantID, sub.EventType, sub.TargetURL,
		sub.CreatedAt, sub.CreatedBy, sub.UpdatedAt, sub.UpdatedBy)
	if err != nil {
		t.Fatalf("insert sub: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 2; i++ {
		entry := &domain.OutboxEntry{
			ID:             uuid.New().String(),
			EventID:        event.ID,
			SubscriptionID: sub.ID,
			Status:         "pending",
			Attempts:       0,
			NextAttemptAt:  now,
			DeliveryID:     uuid.New().String(),
			CreatedAt:      now,
		}
		_, err = outboxRepo.EnqueueEvent(ctx, entry)
		if err != nil {
			t.Fatalf("EnqueueEvent %d: %v", i, err)
		}
	}

	// Add a delivered entry that should not be returned
	delivered := &domain.OutboxEntry{
		ID:             uuid.New().String(),
		EventID:        event.ID,
		SubscriptionID: sub.ID,
		Status:         "delivered",
		Attempts:       1,
		NextAttemptAt:  now,
		DeliveryID:     uuid.New().String(),
		CreatedAt:      now,
	}
	_, err = outboxRepo.EnqueueEvent(ctx, delivered)
	if err != nil {
		t.Fatalf("EnqueueEvent delivered: %v", err)
	}

	entries, err := outboxRepo.GetPendingOutboxEntries(ctx, 10)
	if err != nil {
		t.Fatalf("GetPendingOutboxEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 pending entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Status != "pending" {
			t.Errorf("expected status pending, got %s", e.Status)
		}
	}
}

func TestPostgresOutboxRepository_MarkProcessed(t *testing.T) {
	db := setupTestDB(t)
	eventRepo := repoimpl.NewPostgresEventRepository(db)
	outboxRepo := repoimpl.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")
	_, err := eventRepo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	sub := makeSubscription("tenant-alpha")
	_, err = db.ExecContext(ctx,
		`INSERT INTO subscriptions (id, tenant_id, event_type, target_url, created_at, created_by, updated_at, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		sub.ID, sub.TenantID, sub.EventType, sub.TargetURL,
		sub.CreatedAt, sub.CreatedBy, sub.UpdatedAt, sub.UpdatedBy)
	if err != nil {
		t.Fatalf("insert sub: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	entry := &domain.OutboxEntry{
		ID:             uuid.New().String(),
		EventID:        event.ID,
		SubscriptionID: sub.ID,
		Status:         "pending",
		Attempts:       0,
		NextAttemptAt:  now,
		DeliveryID:     uuid.New().String(),
		CreatedAt:      now,
	}
	_, err = outboxRepo.EnqueueEvent(ctx, entry)
	if err != nil {
		t.Fatalf("EnqueueEvent: %v", err)
	}

	errMsg := "timeout"
	nextAttempt := now.Add(5 * time.Minute)
	err = outboxRepo.MarkOutboxEntryProcessed(ctx, entry.ID, "failed", 1, &nextAttempt, &errMsg)
	if err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	var status string
	var attempts int
	err = db.QueryRowContext(ctx, "SELECT status, attempts FROM outbox WHERE id=$1", entry.ID).Scan(&status, &attempts)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if status != "failed" {
		t.Errorf("status: got %s, want failed", status)
	}
	if attempts != 1 {
		t.Errorf("attempts: got %d, want 1", attempts)
	}
}
