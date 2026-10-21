package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	repoimpl "github.com/mariozul/relaybox/pkg/relaybox/infrastructure/repository"
)

// ============================================================
// TC-ING-01-INT: Successful event ingestion (FR-ING-01, FR-ING-02)
// ============================================================
func TestPostgresEventRepository_CreateEvent_Success(t *testing.T) {
	db := setupTestDB(t)
	eventRepo := repoimpl.NewPostgresEventRepository(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")

	id, err := eventRepo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}
	if id != event.ID {
		t.Errorf("expected id %s, got %s", event.ID, id)
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE id=$1", event.ID).Scan(&count)
	if err != nil {
		t.Fatalf("verify query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

// ============================================================
// TC-ING-02-INT: Idempotent ingestion (FR-ING-03, AC-01)
// ============================================================
func TestPostgresEventRepository_CreateEvent_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	eventRepo := repoimpl.NewPostgresEventRepository(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")

	id1, err := eventRepo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("first CreateEvent: %v", err)
	}

	// Second call with same tenant_id and dedup_key should return the original
	// id AND ErrEventAlreadyExists (idempotent behavior per FR-ING-03).
	id2, err := eventRepo.CreateEvent(ctx, event)
	if err != domain.ErrEventAlreadyExists {
		t.Fatalf("second CreateEvent: expected ErrEventAlreadyExists, got %v", err)
	}
	if id2 != id1 {
		t.Errorf("same dedup key should return original id; got %s, want %s", id2, id1)
	}

	var count int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM events WHERE tenant_id=$1 AND dedup_key=$2",
		event.TenantID, event.DedupKey).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row for dedup pair, got %d", count)
	}
}

// ============================================================
// TC-ING-03-RACE: Concurrent ingestion same dedup_key (AC-05)
// ============================================================
func TestPostgresEventRepository_ConcurrentCreateEvent_Race(t *testing.T) {
	db := setupTestDB(t)
	eventRepo := repoimpl.NewPostgresEventRepository(db)
	ctx := context.Background()

	tenantID := "tenant-race"
	dedupKey := "dedup-race-001"
	now := time.Now().UTC().Truncate(time.Microsecond)

	numGoroutines := 10
	type result struct {
		id  string
		err error
	}
	ch := make(chan result, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			e := &domain.Event{
				ID:        uuid.New().String(),
				TenantID:  tenantID,
				EventType: "order.created",
				Payload:   []byte(`{"order_id":"race-order"}`),
				DedupKey:  dedupKey,
				CreatedAt: now,
				CreatedBy: "test-user",
			}
			id, err := eventRepo.CreateEvent(ctx, e)
			ch <- result{id, err}
		}()
	}

	ids := make(map[string]int)
	errCount := 0
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		r := <-ch
		if r.err != nil {
			if r.err != domain.ErrEventAlreadyExists {
				t.Errorf("unexpected error: %v", r.err)
			}
			errCount++
		} else {
			successCount++
			ids[r.id]++
		}
	}

	// Exactly one should succeed (the INSERT), the rest get ErrEventAlreadyExists
	if successCount != 1 {
		t.Errorf("expected exactly 1 successful insert, got %d", successCount)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 unique event ID stored, got %d: %v", len(ids), ids)
	}
	if errCount != numGoroutines-1 {
		t.Errorf("expected %d ErrEventAlreadyExists, got %d", numGoroutines-1, errCount)
	}

	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM events WHERE tenant_id=$1 AND dedup_key=$2",
		tenantID, dedupKey).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 event row, got %d", count)
	}
}

func TestPostgresEventRepository_GetEventByDedupKey(t *testing.T) {
	db := setupTestDB(t)
	eventRepo := repoimpl.NewPostgresEventRepository(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")
	_, err := eventRepo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	found, err := eventRepo.GetEventByDedupKey(ctx, event.TenantID, event.DedupKey)
	if err != nil {
		t.Fatalf("GetEventByDedupKey: %v", err)
	}
	if found == nil {
		t.Fatal("expected non-nil event")
	}
	if found.ID != event.ID {
		t.Errorf("id: got %s, want %s", found.ID, event.ID)
	}

	notFound, err := eventRepo.GetEventByDedupKey(ctx, "tenant-alpha", "nonexistent")
	if err != nil {
		t.Fatalf("GetEventByDedupKey nonexistent: %v", err)
	}
	if notFound != nil {
		t.Errorf("expected nil for nonexistent, got %v", notFound)
	}
}

// ============================================================
// Transaction Manager tests (RULE-DATA-01)
// ============================================================
func TestPostgresTransactionManager_ExecInTx_Success(t *testing.T) {
	db := setupTestDB(t)
	txManager := repoimpl.NewPostgresTransactionManager(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")

	err := txManager.ExecInTx(ctx, func(txCtx context.Context, tx interface{}) error {
		sqlTx := tx.(*sql.Tx)
		_, err := sqlTx.ExecContext(txCtx,
			`INSERT INTO events (id, tenant_id, event_type, payload, dedup_key, created_at, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			event.ID, event.TenantID, event.EventType, event.Payload,
			event.DedupKey, event.CreatedAt, event.CreatedBy)
		return err
	})
	if err != nil {
		t.Fatalf("ExecInTx: %v", err)
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE id=$1", event.ID).Scan(&count)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after tx commit, got %d", count)
	}
}

func TestPostgresTransactionManager_ExecInTx_Rollback(t *testing.T) {
	db := setupTestDB(t)
	txManager := repoimpl.NewPostgresTransactionManager(db)
	ctx := context.Background()

	event := makeEvent("tenant-alpha")
	expectedErr := fmt.Errorf("simulated rollback")

	err := txManager.ExecInTx(ctx, func(txCtx context.Context, tx interface{}) error {
		sqlTx := tx.(*sql.Tx)
		_, err := sqlTx.ExecContext(txCtx,
			`INSERT INTO events (id, tenant_id, event_type, payload, dedup_key, created_at, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			event.ID, event.TenantID, event.EventType, event.Payload,
			event.DedupKey, event.CreatedAt, event.CreatedBy)
		if err != nil {
			return err
		}
		return expectedErr
	})
	if err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE id=$1", event.ID).Scan(&count)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}
