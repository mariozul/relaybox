package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// ============================================================================
// TC-04-UNIT: Repository interface contract compliance via mock implementations
// ============================================================================

// mockEventRepository implements repository.EventRepository for testing interface contracts.
type mockEventRepository struct {
	events map[string]*domain.Event
}

func newMockEventRepository() *mockEventRepository {
	return &mockEventRepository{
		events: make(map[string]*domain.Event),
	}
}

func (m *mockEventRepository) Save(ctx context.Context, event *domain.Event) (*domain.Event, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	if event == nil {
		return nil, errors.New("event must not be nil")
	}
	m.events[event.ID] = event
	return event, nil
}

func (m *mockEventRepository) FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	for _, e := range m.events {
		if e.TenantID == tenantID && e.DedupKey == dedupKey {
			return e, nil
		}
	}
	return nil, domain.ErrNotFound
}

// Ensure mock implements the interface at compile time.
var _ repository.EventRepository = (*mockEventRepository)(nil)

// mockSubscriptionRepository implements repository.SubscriptionRepository for testing interface contracts.
type mockSubscriptionRepository struct {
	subs map[string]*domain.Subscription
}

func newMockSubscriptionRepository() *mockSubscriptionRepository {
	return &mockSubscriptionRepository{
		subs: make(map[string]*domain.Subscription),
	}
}

func (m *mockSubscriptionRepository) Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	if sub == nil {
		return nil, errors.New("subscription must not be nil")
	}
	m.subs[sub.ID] = sub
	return sub, nil
}

func (m *mockSubscriptionRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	sub, ok := m.subs[id]
	if !ok || sub.TenantID != tenantID {
		return nil, domain.ErrSubscriptionNotFound
	}
	return sub, nil
}

func (m *mockSubscriptionRepository) FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	var result []domain.Subscription
	for _, sub := range m.subs {
		if sub.TenantID == tenantID {
			result = append(result, *sub)
		}
	}
	return result, nil
}

func (m *mockSubscriptionRepository) Delete(ctx context.Context, tenantID, id string) error {
	if ctx == nil {
		return errors.New("context must not be nil")
	}
	sub, ok := m.subs[id]
	if !ok || sub.TenantID != tenantID {
		return domain.ErrSubscriptionNotFound
	}
	delete(m.subs, id)
	return nil
}

// Ensure mock implements the interface at compile time.
var _ repository.SubscriptionRepository = (*mockSubscriptionRepository)(nil)

// mockOutboxRepository implements repository.OutboxRepository for testing interface contracts.
type mockOutboxRepository struct {
	entries map[string]*domain.OutboxEntry
}

func newMockOutboxRepository() *mockOutboxRepository {
	return &mockOutboxRepository{
		entries: make(map[string]*domain.OutboxEntry),
	}
}

func (m *mockOutboxRepository) Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	if entry == nil {
		return nil, errors.New("outbox entry must not be nil")
	}
	m.entries[entry.ID] = entry
	return entry, nil
}

func (m *mockOutboxRepository) FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	var result []domain.OutboxEntry
	for _, entry := range m.entries {
		if entry.Status == domain.OutboxStatusPending || entry.Status == domain.OutboxStatusFailed {
			result = append(result, *entry)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockOutboxRepository) UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error {
	if ctx == nil {
		return errors.New("context must not be nil")
	}
	entry, ok := m.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	entry.Status = status
	return nil
}

// Ensure mock implements the interface at compile time.
var _ repository.OutboxRepository = (*mockOutboxRepository)(nil)

// ============================================================================
// TC-04-UNIT: EventRepository interface contract tests
// ============================================================================

func TestEventRepository_Save(t *testing.T) {
	t.Parallel()

	repo := newMockEventRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	event, err := domain.NewEvent("tenant-abc", "order.created", []byte(`{"id":"123"}`), "dedup-001", "system", now)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	saved, err := repo.Save(ctx, event)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if saved == nil {
		t.Fatal("Save() returned nil event")
	}
	if saved.ID != event.ID {
		t.Errorf("expected ID=%q, got %q", event.ID, saved.ID)
	}
	if saved.TenantID != "tenant-abc" {
		t.Errorf("expected TenantID=%q, got %q", "tenant-abc", saved.TenantID)
	}
}

func TestEventRepository_FindByDedupKey_Found(t *testing.T) {
	t.Parallel()

	repo := newMockEventRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	event, err := domain.NewEvent("tenant-abc", "order.created", []byte(`{"id":"123"}`), "dedup-001", "system", now)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	_, err = repo.Save(ctx, event)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	found, err := repo.FindByDedupKey(ctx, "tenant-abc", "dedup-001")
	if err != nil {
		t.Fatalf("FindByDedupKey() returned error: %v", err)
	}
	if found == nil {
		t.Fatal("FindByDedupKey() returned nil")
	}
	if found.ID != event.ID {
		t.Errorf("expected ID=%q, got %q", event.ID, found.ID)
	}
}

func TestEventRepository_FindByDedupKey_NotFound(t *testing.T) {
	t.Parallel()

	repo := newMockEventRepository()
	ctx := context.Background()

	_, err := repo.FindByDedupKey(ctx, "tenant-abc", "nonexistent")
	if err == nil {
		t.Fatal("FindByDedupKey() expected error for nonexistent key, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEventRepository_FindByDedupKey_DifferentTenant(t *testing.T) {
	t.Parallel()

	repo := newMockEventRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	event, err := domain.NewEvent("tenant-abc", "order.created", []byte(`{"id":"123"}`), "dedup-001", "system", now)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	_, err = repo.Save(ctx, event)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Different tenant with same dedup_key should not find the event.
	_, err = repo.FindByDedupKey(ctx, "tenant-xyz", "dedup-001")
	if err == nil {
		t.Fatal("FindByDedupKey() expected error for different tenant, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ============================================================================
// TC-04-UNIT: SubscriptionRepository interface contract tests
// ============================================================================

func TestSubscriptionRepository_Save(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	saved, err := repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if saved == nil {
		t.Fatal("Save() returned nil subscription")
	}
	if saved.ID != sub.ID {
		t.Errorf("expected ID=%q, got %q", sub.ID, saved.ID)
	}
}

func TestSubscriptionRepository_FindByID_Found(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	found, err := repo.FindByID(ctx, "tenant-abc", sub.ID)
	if err != nil {
		t.Fatalf("FindByID() returned error: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID() returned nil")
	}
	if found.ID != sub.ID {
		t.Errorf("expected ID=%q, got %q", sub.ID, found.ID)
	}
}

func TestSubscriptionRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "tenant-abc", "nonexistent")
	if err == nil {
		t.Fatal("FindByID() expected error for nonexistent subscription, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionRepository_FindByID_DifferentTenant(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Tenant xyz should not be able to find tenant-abc's subscription.
	_, err = repo.FindByID(ctx, "tenant-xyz", sub.ID)
	if err == nil {
		t.Fatal("FindByID() expected error for different tenant, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionRepository_FindByTenantID(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	sub1, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook1", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	sub2, err := domain.NewSubscription("tenant-abc", "order.shipped", "https://example.com/webhook2", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	sub3, err := domain.NewSubscription("tenant-xyz", "order.created", "https://other.com/webhook", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	_, err = repo.Save(ctx, sub1)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	_, err = repo.Save(ctx, sub2)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	_, err = repo.Save(ctx, sub3)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	subs, err := repo.FindByTenantID(ctx, "tenant-abc")
	if err != nil {
		t.Fatalf("FindByTenantID() returned error: %v", err)
	}

	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions for tenant-abc, got %d", len(subs))
	}
}

func TestSubscriptionRepository_Delete_Success(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	err = repo.Delete(ctx, "tenant-abc", sub.ID)
	if err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	// Verify it's gone.
	_, err = repo.FindByID(ctx, "tenant-abc", sub.ID)
	if err == nil {
		t.Fatal("expected subscription to be deleted, but FindByID returned no error")
	}
}

func TestSubscriptionRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()

	err := repo.Delete(ctx, "tenant-abc", "nonexistent")
	if err == nil {
		t.Fatal("Delete() expected error for nonexistent subscription, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionRepository_Delete_DifferentTenant(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = repo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Tenant xyz should not be able to delete tenant-abc's subscription.
	err = repo.Delete(ctx, "tenant-xyz", sub.ID)
	if err == nil {
		t.Fatal("Delete() expected error for different tenant, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// ============================================================================
// TC-04-UNIT: OutboxRepository interface contract tests
// ============================================================================

func TestOutboxRepository_Save(t *testing.T) {
	t.Parallel()

	repo := newMockOutboxRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	entry, err := domain.NewOutboxEntry("evt-001", "sub-001", now)
	if err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}

	saved, err := repo.Save(ctx, entry)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if saved == nil {
		t.Fatal("Save() returned nil entry")
	}
	if saved.ID != entry.ID {
		t.Errorf("expected ID=%q, got %q", entry.ID, saved.ID)
	}
}

func TestOutboxRepository_FindPending(t *testing.T) {
	t.Parallel()

	repo := newMockOutboxRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	entry1, err := domain.NewOutboxEntry("evt-001", "sub-001", now)
	if err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}
	entry2, err := domain.NewOutboxEntry("evt-002", "sub-002", now)
	if err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}
	entry3 := &domain.OutboxEntry{
		ID:             "delivered-001",
		EventID:        "evt-003",
		SubscriptionID: "sub-003",
		Status:         domain.OutboxStatusDelivered,
		CreatedAt:      now,
	}

	_, err = repo.Save(ctx, entry1)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	_, err = repo.Save(ctx, entry2)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	_, err = repo.Save(ctx, entry3)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	pending, err := repo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("expected 2 pending entries, got %d", len(pending))
	}
}

func TestOutboxRepository_FindPending_WithLimit(t *testing.T) {
	t.Parallel()

	repo := newMockOutboxRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		entry, err := domain.NewOutboxEntry(
			"evt-00"+string(rune('0'+i)),
			"sub-00"+string(rune('0'+i)),
			now,
		)
		if err != nil {
			t.Fatalf("failed to create outbox entry: %v", err)
		}
		_, err = repo.Save(ctx, entry)
		if err != nil {
			t.Fatalf("Save() returned error: %v", err)
		}
	}

	pending, err := repo.FindPending(ctx, 3)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}

	if len(pending) > 3 {
		t.Errorf("expected at most 3 pending entries with limit=3, got %d", len(pending))
	}
}

func TestOutboxRepository_UpdateStatus(t *testing.T) {
	t.Parallel()

	repo := newMockOutboxRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	entry, err := domain.NewOutboxEntry("evt-001", "sub-001", now)
	if err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}
	_, err = repo.Save(ctx, entry)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	err = repo.UpdateStatus(ctx, entry.ID, domain.OutboxStatusDelivered)
	if err != nil {
		t.Fatalf("UpdateStatus() returned error: %v", err)
	}

	// Verify status was updated by checking pending no longer includes it.
	pending, _ := repo.FindPending(ctx, 10)
	for _, p := range pending {
		if p.ID == entry.ID {
			t.Errorf("expected entry %s to be excluded from pending after delivery", entry.ID)
		}
	}
}

func TestOutboxRepository_UpdateStatus_NotFound(t *testing.T) {
	t.Parallel()

	repo := newMockOutboxRepository()
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, "nonexistent", domain.OutboxStatusDelivered)
	if err == nil {
		t.Fatal("UpdateStatus() expected error for nonexistent entry, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ============================================================================
// TC-02-EDGE: Boundary tests for repository interfaces - nil context handling
// These tests verify that mock implementations gracefully handle nil context
// rather than panicking. Production code should never pass nil context
// (RULE-RES-01), but defensive behavior in mocks prevents test flakiness.
// ============================================================================

func TestEventRepository_ContextNil(t *testing.T) {
	t.Parallel()

	repo := newMockEventRepository()

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	_, err := repo.Save(nil, &domain.Event{})
	if err == nil {
		t.Error("Save() expected error when ctx is nil")
	}

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	_, err = repo.FindByDedupKey(nil, "tenant", "key")
	if err == nil {
		t.Error("FindByDedupKey() expected error when ctx is nil")
	}
}

func TestSubscriptionRepository_ContextNil(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	_, err := repo.Save(nil, &domain.Subscription{})
	if err == nil {
		t.Error("Save() expected error when ctx is nil")
	}

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	_, err = repo.FindByID(nil, "tenant", "id")
	if err == nil {
		t.Error("FindByID() expected error when ctx is nil")
	}

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	_, err = repo.FindByTenantID(nil, "tenant")
	if err == nil {
		t.Error("FindByTenantID() expected error when ctx is nil")
	}

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	err = repo.Delete(nil, "tenant", "id")
	if err == nil {
		t.Error("Delete() expected error when ctx is nil")
	}
}

func TestOutboxRepository_ContextNil(t *testing.T) {
	t.Parallel()

	repo := newMockOutboxRepository()

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	_, err := repo.Save(nil, &domain.OutboxEntry{})
	if err == nil {
		t.Error("Save() expected error when ctx is nil")
	}

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	_, err = repo.FindPending(nil, 10)
	if err == nil {
		t.Error("FindPending() expected error when ctx is nil")
	}

	//nolint:staticcheck // SA1012: intentional nil context test for defensive behavior
	err = repo.UpdateStatus(nil, "id", domain.OutboxStatusDelivered)
	if err == nil {
		t.Error("UpdateStatus() expected error when ctx is nil")
	}
}
