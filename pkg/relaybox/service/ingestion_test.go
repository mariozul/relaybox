package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/dto"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// ---------------------------------------------------------------------------
// Mock implementations — we fulfill the repository interfaces locally to keep
// the test hermetic (no DB dependency) and verify the service logic in
// isolation (Clean Architecture / unit test boundary).
// ---------------------------------------------------------------------------

type mockEventRepo struct {
	mu           sync.Mutex
	eventsByKey  map[string]*domain.Event // key = tenantID + "|" + dedupKey
	eventsByID   map[string]*domain.Event // key = event ID
	createCalled int
}

func newMockEventRepo() *mockEventRepo {
	return &mockEventRepo{
		eventsByKey: make(map[string]*domain.Event),
		eventsByID:  make(map[string]*domain.Event),
	}
}

func (m *mockEventRepo) mkKey(tenantID, dedupKey string) string {
	return tenantID + "|" + dedupKey
}

func (m *mockEventRepo) CreateEvent(ctx context.Context, event *domain.Event) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalled++

	key := m.mkKey(event.TenantID, event.DedupKey)
	if existing, ok := m.eventsByKey[key]; ok {
		// Simulate unique constraint violation — return existing ID + sentinel error
		return existing.ID, domain.ErrEventAlreadyExists
	}
	m.eventsByKey[key] = event
	m.eventsByID[event.ID] = event
	return event.ID, nil
}

func (m *mockEventRepo) GetEventByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.mkKey(tenantID, dedupKey)
	ev, ok := m.eventsByKey[key]
	if !ok {
		return nil, nil
	}
	return ev, nil
}

func (m *mockEventRepo) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev, ok := m.eventsByID[id]
	if !ok {
		return nil, nil
	}
	return ev, nil
}

type mockOutboxRepo struct {
	mu            sync.Mutex
	entries       []domain.OutboxEntry
	enqueueCalled int
}

func newMockOutboxRepo() *mockOutboxRepo {
	return &mockOutboxRepo{}
}

func (m *mockOutboxRepo) EnqueueEvent(ctx context.Context, entry *domain.OutboxEntry) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueueCalled++
	m.entries = append(m.entries, *entry)
	return entry.ID, nil
}

func (m *mockOutboxRepo) GetPendingOutboxEntries(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	return nil, nil // not needed for ingestion tests
}

func (m *mockOutboxRepo) MarkOutboxEntryProcessed(ctx context.Context, id string, status string, attempts int, nextAttemptAt *time.Time, lastAttemptError *string) error {
	return nil // not needed for ingestion tests
}

type mockTxManager struct {
	mu          sync.Mutex
	beginCalled int
	commitCnt   int
	rollbackCnt int
}

func newMockTxManager() *mockTxManager {
	return &mockTxManager{}
}

func (m *mockTxManager) BeginTx(ctx context.Context) (interface{}, error) {
	m.mu.Lock()
	m.beginCalled++
	m.mu.Unlock()
	return &mockTx{}, nil
}

func (m *mockTxManager) CommitTx(ctx context.Context, tx interface{}) error {
	m.mu.Lock()
	m.commitCnt++
	m.mu.Unlock()
	return nil
}

func (m *mockTxManager) RollbackTx(ctx context.Context, tx interface{}) error {
	m.mu.Lock()
	m.rollbackCnt++
	m.mu.Unlock()
	return nil
}

func (m *mockTxManager) ExecInTx(ctx context.Context, fn func(txCtx context.Context, tx interface{}) error) error {
	tx := &mockTx{}
	err := fn(ctx, tx)
	if err != nil {
		m.mu.Lock()
		m.rollbackCnt++
		m.mu.Unlock()
		return err
	}
	m.mu.Lock()
	m.commitCnt++
	m.mu.Unlock()
	return nil
}

type mockTx struct{}

// clock is a deterministic time provider for tests.
var testClock = func() time.Time {
	return time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// TC-ING-01-INT: Successful event ingestion with outbox enqueue
// ---------------------------------------------------------------------------
func TestIngestEvent_Success(t *testing.T) {
	t.Parallel()

	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	txManager := newMockTxManager()

	svc := service.NewEventIngestionService(eventRepo, outboxRepo, txManager, testClock)

	req := &dto.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-abc-001",
	}

	eventID, err := svc.IngestEvent(context.Background(), "tenant-alpha", req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if eventID == "" {
		t.Fatal("expected non-empty event ID")
	}

	// Verify: event was created via eventRepo
	if eventRepo.createCalled != 1 {
		t.Errorf("expected eventRepo.CreateEvent called 1 time, got %d", eventRepo.createCalled)
	}

	// Verify: outbox entry was enqueued
	if outboxRepo.enqueueCalled != 1 {
		t.Errorf("expected outboxRepo.EnqueueEvent called 1 time, got %d", outboxRepo.enqueueCalled)
	}

	// Verify: transaction was committed
	if txManager.commitCnt != 1 {
		t.Errorf("expected 1 commit, got %d", txManager.commitCnt)
	}
	if txManager.rollbackCnt != 0 {
		t.Errorf("expected 0 rollbacks, got %d", txManager.rollbackCnt)
	}
}

// ---------------------------------------------------------------------------
// TC-ING-02-INT: Idempotent event ingestion prevents duplicates
// (FR-ING-03, AC-01)
// ---------------------------------------------------------------------------
func TestIngestEvent_IdempotentSameDedupKey(t *testing.T) {
	t.Parallel()

	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	txManager := newMockTxManager()

	svc := service.NewEventIngestionService(eventRepo, outboxRepo, txManager, testClock)

	req := &dto.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-abc-002",
	}

	// First ingestion — should succeed
	firstID, err := svc.IngestEvent(context.Background(), "tenant-alpha", req)
	if err != nil {
		t.Fatalf("first ingestion: expected no error, got: %v", err)
	}
	if firstID == "" {
		t.Fatal("first ingestion: expected non-empty ID")
	}

	// Second ingestion with same (tenant, dedup_key) — should return same ID, no error
	secondID, err := svc.IngestEvent(context.Background(), "tenant-alpha", req)
	if err != nil {
		t.Fatalf("second ingestion: expected no error (idempotent), got: %v", err)
	}
	if secondID != firstID {
		t.Errorf("second ingestion: expected same event ID %q, got %q", firstID, secondID)
	}

	// Verify: eventRepo.CreateEvent was called twice
	if eventRepo.createCalled != 2 {
		t.Errorf("expected eventRepo.CreateEvent called 2 times, got %d", eventRepo.createCalled)
	}

	// Verify: only 1 unique event persisted
	eventRepo.mu.Lock()
	count := len(eventRepo.eventsByKey)
	eventRepo.mu.Unlock()
	if count != 1 {
		t.Errorf("expected exactly 1 unique event key in repo, got %d", count)
	}

	// Verify no rollback
	if txManager.rollbackCnt != 0 {
		t.Errorf("expected 0 rollbacks for idempotent flow, got %d", txManager.rollbackCnt)
	}
}

// ---------------------------------------------------------------------------
// TC-ING-02-INT variant: different tenant with same dedup_key is NOT a duplicate
// ---------------------------------------------------------------------------
func TestIngestEvent_SameDedupKeyDifferentTenant(t *testing.T) {
	t.Parallel()

	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	txManager := newMockTxManager()

	svc := service.NewEventIngestionService(eventRepo, outboxRepo, txManager, testClock)

	req := &dto.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-shared",
	}

	// Ingest for tenant-alpha
	idAlpha, err := svc.IngestEvent(context.Background(), "tenant-alpha", req)
	if err != nil {
		t.Fatalf("tenant-alpha: expected no error, got: %v", err)
	}

	// Ingest same dedup_key for tenant-beta — should be a new event
	idBeta, err := svc.IngestEvent(context.Background(), "tenant-beta", req)
	if err != nil {
		t.Fatalf("tenant-beta: expected no error, got: %v", err)
	}

	if idAlpha == idBeta {
		t.Error("expected different event IDs for different tenants with same dedup_key")
	}

	eventRepo.mu.Lock()
	count := len(eventRepo.eventsByKey)
	eventRepo.mu.Unlock()
	if count != 2 {
		t.Errorf("expected 2 unique event entries (one per tenant), got %d", count)
	}
}

// ---------------------------------------------------------------------------
// TC-ING-03-RACE: Concurrent ingestion with same dedup_key passes race detector
// (FR-ING-03, AC-05)
// ---------------------------------------------------------------------------
func TestIngestEvent_ConcurrentIdempotentRace(t *testing.T) {
	t.Parallel()

	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	txManager := newMockTxManager()

	svc := service.NewEventIngestionService(eventRepo, outboxRepo, txManager, testClock)

	req := &dto.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"race-test"}`),
		DedupKey:  "dedup-race-001",
	}

	const goroutines = 10
	var wg sync.WaitGroup
	ids := make([]string, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id, err := svc.IngestEvent(context.Background(), "tenant-race", req)
			ids[idx] = id
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// Collect all non-empty IDs
	idSet := make(map[string]int)
	for _, id := range ids {
		if id != "" {
			idSet[id]++
		}
	}

	// All successful calls must return the SAME event ID (idempotent)
	if len(idSet) == 0 {
		t.Fatal("expected at least one successful ingestion, got none")
	}
	if len(idSet) > 1 {
		t.Errorf("expected exactly 1 unique event ID across %d goroutines, got %d: %v",
			goroutines, len(idSet), idSet)
	}

	// Verify only 1 event persisted
	eventRepo.mu.Lock()
	count := len(eventRepo.eventsByKey)
	eventRepo.mu.Unlock()
	if count != 1 {
		t.Errorf("expected exactly 1 event persisted, got %d", count)
	}

	t.Logf("Concurrent idempotent test: %d goroutines, %d unique IDs, %d total persisted keys",
		goroutines, len(idSet), count)
}

// ---------------------------------------------------------------------------
// Edge case: nil/empty inputs handled gracefully
// ---------------------------------------------------------------------------
func TestIngestEvent_NilRequest(t *testing.T) {
	t.Parallel()

	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	txManager := newMockTxManager()

	svc := service.NewEventIngestionService(eventRepo, outboxRepo, txManager, testClock)

	_, err := svc.IngestEvent(context.Background(), "tenant-alpha", nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Edge case: empty tenant ID
// ---------------------------------------------------------------------------
func TestIngestEvent_EmptyTenant(t *testing.T) {
	t.Parallel()

	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	txManager := newMockTxManager()

	svc := service.NewEventIngestionService(eventRepo, outboxRepo, txManager, testClock)

	req := &dto.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{}`),
		DedupKey:  "dedup-empty-tenant",
	}

	_, err := svc.IngestEvent(context.Background(), "", req)
	if err == nil {
		t.Fatal("expected error for empty tenant ID, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}
