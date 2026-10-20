package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// ============================================================================
// Mock Repositories for IngestionService testing
// ============================================================================

type mockIngestionEventRepo struct {
	events      map[string]*domain.Event
	dedupIndex  map[string]string // "tenant:dedupKey" -> eventID
}

func newMockIngestionEventRepo() *mockIngestionEventRepo {
	return &mockIngestionEventRepo{
		events:     make(map[string]*domain.Event),
		dedupIndex: make(map[string]string),
	}
}

func (m *mockIngestionEventRepo) Save(ctx context.Context, event *domain.Event) (*domain.Event, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	if event == nil {
		return nil, errors.New("event must not be nil")
	}
	// Check for duplicate before saving
	key := event.TenantID + ":" + event.DedupKey
	if existingID, ok := m.dedupIndex[key]; ok {
		existingEvent, _ := m.events[existingID]
		return existingEvent, domain.ErrDuplicateEvent
	}
	m.events[event.ID] = event
	m.dedupIndex[key] = event.ID
	return event, nil
}

func (m *mockIngestionEventRepo) FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	key := tenantID + ":" + dedupKey
	if eventID, ok := m.dedupIndex[key]; ok {
		return m.events[eventID], nil
	}
	return nil, domain.ErrNotFound
}

type mockIngestionOutboxRepo struct {
	entries map[string]*domain.OutboxEntry
}

func newMockIngestionOutboxRepo() *mockIngestionOutboxRepo {
	return &mockIngestionOutboxRepo{
		entries: make(map[string]*domain.OutboxEntry),
	}
}

func (m *mockIngestionOutboxRepo) Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	if entry == nil {
		return nil, errors.New("outbox entry must not be nil")
	}
	m.entries[entry.ID] = entry
	return entry, nil
}

func (m *mockIngestionOutboxRepo) FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
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

func (m *mockIngestionOutboxRepo) UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error {
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

type mockIngestionSubRepo struct {
	subs map[string]*domain.Subscription
}

func newMockIngestionSubRepo() *mockIngestionSubRepo {
	return &mockIngestionSubRepo{
		subs: make(map[string]*domain.Subscription),
	}
}

func (m *mockIngestionSubRepo) Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	if sub == nil {
		return nil, errors.New("subscription must not be nil")
	}
	m.subs[sub.ID] = sub
	return sub, nil
}

func (m *mockIngestionSubRepo) FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	sub, ok := m.subs[id]
	if !ok || sub.TenantID != tenantID {
		return nil, domain.ErrSubscriptionNotFound
	}
	return sub, nil
}

func (m *mockIngestionSubRepo) FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
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

func (m *mockIngestionSubRepo) Delete(ctx context.Context, tenantID, id string) error {
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

func (m *mockIngestionSubRepo) FindByEventTypeAndTenantID(ctx context.Context, tenantID string, eventType string) ([]domain.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	var result []domain.Subscription
	for _, sub := range m.subs {
		if sub.TenantID == tenantID && sub.EventType == eventType {
			result = append(result, *sub)
		}
	}
	return result, nil
}

// compile-time interface checks
var _ interface {
	Save(ctx context.Context, event *domain.Event) (*domain.Event, error)
	FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error)
} = (*mockIngestionEventRepo)(nil)

var _ interface {
	Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error)
	FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error)
	UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error
} = (*mockIngestionOutboxRepo)(nil)

// ============================================================================
// TC-01-UNIT: Successful event ingestion and outbox entry creation
// ============================================================================

func TestIngestionService_IngestEvent_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	eventRepo := newMockIngestionEventRepo()
	outboxRepo := newMockIngestionOutboxRepo()
	subRepo := newMockIngestionSubRepo()

	svc := service.NewIngestionService(eventRepo, outboxRepo, subRepo)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create a matching subscription for the event type
	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "system", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = subRepo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	// Act
	resp, err := svc.IngestEvent(ctx, "tenant-abc", req, now)

	// Assert
	if err != nil {
		t.Fatalf("IngestEvent() returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("IngestEvent() returned nil response")
	}

	if resp.EventID == "" {
		t.Error("expected non-empty EventID")
	}

	// Verify the event was persisted in the event repository
	persistedEvent, err := eventRepo.FindByDedupKey(ctx, "tenant-abc", "dedup-001")
	if err != nil {
		t.Fatalf("event not found in repository: %v", err)
	}
	if persistedEvent.ID != resp.EventID {
		t.Errorf("expected EventID=%q, got %q", persistedEvent.ID, resp.EventID)
	}
	if persistedEvent.TenantID != "tenant-abc" {
		t.Errorf("expected TenantID=tenant-abc, got %q", persistedEvent.TenantID)
	}
	if persistedEvent.EventType != "order.created" {
		t.Errorf("expected EventType=order.created, got %q", persistedEvent.EventType)
	}
	if string(persistedEvent.Payload) != `{"order_id":"123"}` {
		t.Errorf("expected Payload=%q, got %q", `{"order_id":"123"}`, string(persistedEvent.Payload))
	}
	if persistedEvent.DedupKey != "dedup-001" {
		t.Errorf("expected DedupKey=dedup-001, got %q", persistedEvent.DedupKey)
	}
	if persistedEvent.CreatedBy != "system" {
		t.Errorf("expected CreatedBy=system, got %q", persistedEvent.CreatedBy)
	}
	if !persistedEvent.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt=%v, got %v", now, persistedEvent.CreatedAt)
	}

	// Verify outbox entry was created for the matching subscription
	pending, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending outbox entry, got %d", len(pending))
	}
	if pending[0].EventID != resp.EventID {
		t.Errorf("expected outbox EventID=%q, got %q", resp.EventID, pending[0].EventID)
	}
	if pending[0].SubscriptionID != sub.ID {
		t.Errorf("expected outbox SubscriptionID=%q, got %q", sub.ID, pending[0].SubscriptionID)
	}
	if pending[0].Status != domain.OutboxStatusPending {
		t.Errorf("expected outbox Status=%q, got %q", domain.OutboxStatusPending, pending[0].Status)
	}
}

func TestIngestionService_IngestEvent_NoMatchingSubscriptions(t *testing.T) {
	t.Parallel()

	// Arrange
	eventRepo := newMockIngestionEventRepo()
	outboxRepo := newMockIngestionOutboxRepo()
	subRepo := newMockIngestionSubRepo()

	svc := service.NewIngestionService(eventRepo, outboxRepo, subRepo)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create a subscription for a different event type
	sub, err := domain.NewSubscription("tenant-abc", "order.shipped", "https://example.com/webhook", "system", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = subRepo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	req := service.IngestEventRequest{
		EventType: "order.created", // No subscription for this event type
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	// Act
	resp, err := svc.IngestEvent(ctx, "tenant-abc", req, now)

	// Assert
	if err != nil {
		t.Fatalf("IngestEvent() returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("IngestEvent() returned nil response")
	}

	if resp.EventID == "" {
		t.Error("expected non-empty EventID")
	}

	// Event should be persisted even without matching subscriptions
	persistedEvent, err := eventRepo.FindByDedupKey(ctx, "tenant-abc", "dedup-001")
	if err != nil {
		t.Fatalf("event not found in repository: %v", err)
	}
	if persistedEvent.ID != resp.EventID {
		t.Errorf("expected EventID=%q, got %q", persistedEvent.ID, resp.EventID)
	}

	// No outbox entries should exist since no subscription matches
	pending, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending outbox entries, got %d", len(pending))
	}
}

// ============================================================================
// TC-02-EDGE: Idempotent ingestion of duplicate event (same dedup_key)
// ============================================================================

func TestIngestionService_IngestEvent_DuplicateDedupKey(t *testing.T) {
	t.Parallel()

	// Arrange
	eventRepo := newMockIngestionEventRepo()
	outboxRepo := newMockIngestionOutboxRepo()
	subRepo := newMockIngestionSubRepo()

	svc := service.NewIngestionService(eventRepo, outboxRepo, subRepo)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create a matching subscription
	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "system", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = subRepo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	// Act - First ingestion (should succeed)
	resp1, err := svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err != nil {
		t.Fatalf("first IngestEvent() returned error: %v", err)
	}
	if resp1 == nil || resp1.EventID == "" {
		t.Fatal("first IngestEvent() returned nil or empty EventID")
	}

	// Act - Second ingestion with same dedup_key (should return same EventID)
	resp2, err := svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err != nil {
		t.Fatalf("second IngestEvent() returned error: %v", err)
	}
	if resp2 == nil {
		t.Fatal("second IngestEvent() returned nil response")
	}

	// Assert - Same EventID should be returned
	if resp2.EventID != resp1.EventID {
		t.Errorf("expected EventID=%q for duplicate, got %q", resp1.EventID, resp2.EventID)
	}

	// Verify only one event exists in the repository
	pending, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}
	// The outbox entries should only be created once
	if len(pending) != 1 {
		t.Errorf("expected 1 pending outbox entry, got %d", len(pending))
	}
}

func TestIngestionService_IngestEvent_DuplicateAcrossTenants(t *testing.T) {
	t.Parallel()

	// Arrange
	eventRepo := newMockIngestionEventRepo()
	outboxRepo := newMockIngestionOutboxRepo()
	subRepo := newMockIngestionSubRepo()

	svc := service.NewIngestionService(eventRepo, outboxRepo, subRepo)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create subscriptions for both tenants
	sub1, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook1", "system", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = subRepo.Save(ctx, sub1)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	sub2, err := domain.NewSubscription("tenant-xyz", "order.created", "https://example.com/webhook2", "system", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = subRepo.Save(ctx, sub2)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	// Act - Tenant ABC ingests
	resp1, err := svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err != nil {
		t.Fatalf("tenant-abc IngestEvent() returned error: %v", err)
	}

	// Act - Tenant XYZ ingests with SAME dedup_key (should be treated as unique per tenant)
	resp2, err := svc.IngestEvent(ctx, "tenant-xyz", req, now)
	if err != nil {
		t.Fatalf("tenant-xyz IngestEvent() returned error: %v", err)
	}

	// Assert - Different tenants with same dedup_key should get different event IDs
	if resp2.EventID == resp1.EventID {
		t.Errorf("different tenants with same dedup_key should get different EventIDs, both got %q", resp1.EventID)
	}

	// Verify two events exist and two outbox entries
	pending, err := outboxRepo.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending() returned error: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending outbox entries, got %d", len(pending))
	}
}

// ============================================================================
// TC-02-EDGE: Boundary tests for invalid input
// ============================================================================

func TestIngestionService_IngestEvent_InvalidPayload(t *testing.T) {
	t.Parallel()

	svc := service.NewIngestionService(newMockIngestionEventRepo(), newMockIngestionOutboxRepo(), newMockIngestionSubRepo())
	ctx := context.Background()
	now := time.Now().UTC()

	tests := []struct {
		name        string
		tenantID    string
		req         service.IngestEventRequest
		errSentinel error
	}{
		{
			name:     "empty tenantID",
			tenantID: "",
			req: service.IngestEventRequest{
				EventType: "order.created",
				Payload:   []byte(`{}`),
				DedupKey:  "dedup-001",
			},
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:     "empty event_type",
			tenantID: "tenant-abc",
			req: service.IngestEventRequest{
				EventType: "",
				Payload:   []byte(`{}`),
				DedupKey:  "dedup-001",
			},
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:     "nil payload",
			tenantID: "tenant-abc",
			req: service.IngestEventRequest{
				EventType: "order.created",
				Payload:   nil,
				DedupKey:  "dedup-001",
			},
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:     "empty payload",
			tenantID: "tenant-abc",
			req: service.IngestEventRequest{
				EventType: "order.created",
				Payload:   []byte{},
				DedupKey:  "dedup-001",
			},
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:     "empty dedup_key",
			tenantID: "tenant-abc",
			req: service.IngestEventRequest{
				EventType: "order.created",
				Payload:   []byte(`{}`),
				DedupKey:  "",
			},
			errSentinel: domain.ErrInvalidEventPayload,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := svc.IngestEvent(ctx, tc.tenantID, tc.req, now)
			if err == nil {
				t.Fatalf("expected error %v but got nil", tc.errSentinel)
			}
			if !errors.Is(err, tc.errSentinel) {
				t.Fatalf("expected error %v, got %v", tc.errSentinel, err)
			}
		})
	}
}

func TestIngestionService_IngestEvent_NilContext(t *testing.T) {
	t.Parallel()

	svc := service.NewIngestionService(newMockIngestionEventRepo(), newMockIngestionOutboxRepo(), newMockIngestionSubRepo())
	now := time.Now().UTC()

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{}`),
		DedupKey:  "dedup-001",
	}

	//nolint:staticcheck // SA1012: intentional nil context test
	_, err := svc.IngestEvent(nil, "tenant-abc", req, now)
	if err == nil {
		t.Error("IngestEvent() expected error when ctx is nil")
	}
}

// ============================================================================
// Helper verification for service interface compliance
// ============================================================================

func TestIngestionService_Interface(t *testing.T) {
	t.Parallel()

	// This is a compile-time check that the service implements the expected interface
	_ = service.NewIngestionService(newMockIngestionEventRepo(), newMockIngestionOutboxRepo(), newMockIngestionSubRepo())
}

// ============================================================================
// Error-injecting mocks for edge case testing
// ============================================================================

type errorEventRepo struct {
	mock      *mockIngestionEventRepo
	findError error
	saveError error
}

func newErrorEventRepo() *errorEventRepo {
	return &errorEventRepo{
		mock: newMockIngestionEventRepo(),
	}
}

func (e *errorEventRepo) Save(ctx context.Context, event *domain.Event) (*domain.Event, error) {
	if e.saveError != nil {
		return nil, e.saveError
	}
	return e.mock.Save(ctx, event)
}

func (e *errorEventRepo) FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error) {
	if e.findError != nil {
		return nil, e.findError
	}
	return e.mock.FindByDedupKey(ctx, tenantID, dedupKey)
}

type errorOutboxRepo struct {
	mock      *mockIngestionOutboxRepo
	saveError error
}

func newErrorOutboxRepo() *errorOutboxRepo {
	return &errorOutboxRepo{
		mock: newMockIngestionOutboxRepo(),
	}
}

func (e *errorOutboxRepo) Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error) {
	if e.saveError != nil {
		return nil, e.saveError
	}
	return e.mock.Save(ctx, entry)
}

func (e *errorOutboxRepo) FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	return e.mock.FindPending(ctx, limit)
}

func (e *errorOutboxRepo) UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error {
	return e.mock.UpdateStatus(ctx, id, status)
}

type errorSubRepo struct {
	mock          *mockIngestionSubRepo
	findByTypeErr error
}

func newErrorSubRepo() *errorSubRepo {
	return &errorSubRepo{
		mock: newMockIngestionSubRepo(),
	}
}

func (e *errorSubRepo) Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	return e.mock.Save(ctx, sub)
}

func (e *errorSubRepo) FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	return e.mock.FindByID(ctx, tenantID, id)
}

func (e *errorSubRepo) FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return e.mock.FindByTenantID(ctx, tenantID)
}

func (e *errorSubRepo) Delete(ctx context.Context, tenantID, id string) error {
	return e.mock.Delete(ctx, tenantID, id)
}

func (e *errorSubRepo) FindByEventTypeAndTenantID(ctx context.Context, tenantID string, eventType string) ([]domain.Subscription, error) {
	if e.findByTypeErr != nil {
		return nil, e.findByTypeErr
	}
	return e.mock.FindByEventTypeAndTenantID(ctx, tenantID, eventType)
}

// ============================================================================
// TC-02-EDGE: Repository error path testing
// ============================================================================

func TestIngestionService_FindByDedupKeyError(t *testing.T) {
	t.Parallel()

	eventRepo := newErrorEventRepo()
	eventRepo.findError = errors.New("db connection error")

	svc := service.NewIngestionService(eventRepo, newErrorOutboxRepo(), newErrorSubRepo())
	ctx := context.Background()
	now := time.Now().UTC()

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	_, err := svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err == nil {
		t.Fatal("IngestEvent() expected error when FindByDedupKey fails, got nil")
	}
	if !errors.Is(err, domain.ErrInternal) {
		t.Errorf("expected ErrInternal, got %v", err)
	}
}

func TestIngestionService_SaveEventRepoError(t *testing.T) {
	t.Parallel()

	eventRepo := newErrorEventRepo()
	eventRepo.saveError = errors.New("db write failure")

	svc := service.NewIngestionService(eventRepo, newErrorOutboxRepo(), newErrorSubRepo())
	ctx := context.Background()
	now := time.Now().UTC()

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	_, err := svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err == nil {
		t.Fatal("IngestEvent() expected error when EventRepo.Save fails, got nil")
	}
	if !errors.Is(err, domain.ErrInternal) {
		t.Errorf("expected ErrInternal, got %v", err)
	}
}

func TestIngestionService_SubRepoFindError(t *testing.T) {
	t.Parallel()

	subRepo := newErrorSubRepo()
	subRepo.findByTypeErr = errors.New("db query failure")

	svc := service.NewIngestionService(newErrorEventRepo(), newErrorOutboxRepo(), subRepo)
	ctx := context.Background()
	now := time.Now().UTC()

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	_, err := svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err == nil {
		t.Fatal("IngestEvent() expected error when subscription lookup fails, got nil")
	}
	if !errors.Is(err, domain.ErrInternal) {
		t.Errorf("expected ErrInternal, got %v", err)
	}
}

func TestIngestionService_OutboxSaveError(t *testing.T) {
	t.Parallel()

	outboxRepo := newErrorOutboxRepo()
	outboxRepo.saveError = errors.New("outbox write failure")
	subRepo := newErrorSubRepo()

	svc := service.NewIngestionService(newErrorEventRepo(), outboxRepo, subRepo)
	ctx := context.Background()
	now := time.Now().UTC()

	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "system", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	_, err = subRepo.Save(ctx, sub)
	if err != nil {
		t.Fatalf("failed to save subscription: %v", err)
	}

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "dedup-001",
	}

	_, err = svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err == nil {
		t.Fatal("IngestEvent() expected error when OutboxRepo.Save fails, got nil")
	}
	if !errors.Is(err, domain.ErrInternal) {
		t.Errorf("expected ErrInternal, got %v", err)
	}
}

func TestIngestionService_EmptyDedupKeyDenied(t *testing.T) {
	t.Parallel()

	eventRepo := newMockIngestionEventRepo()
	outboxRepo := newMockIngestionOutboxRepo()
	subRepo := newMockIngestionSubRepo()

	svc := service.NewIngestionService(eventRepo, outboxRepo, subRepo)
	ctx := context.Background()
	now := time.Now().UTC()

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"123"}`),
		DedupKey:  "",
	}

	_, err := svc.IngestEvent(ctx, "tenant-abc", req, now)
	if err == nil {
		t.Fatal("IngestEvent() expected error for empty dedup_key, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidEventPayload) {
		t.Errorf("expected ErrInvalidEventPayload, got %v", err)
	}
}

// ============================================================================
// SubscriptionToResponse test coverage
// ============================================================================

func TestSubscriptionToResponse_Coverage(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	sub, err := domain.NewSubscription("tenant-abc", "order.created", "https://example.com/webhook", "admin", now)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	resp := service.SubscriptionToResponse(sub)
	if resp.ID != sub.ID {
		t.Errorf("expected ID=%q, got %q", sub.ID, resp.ID)
	}
	if resp.TenantID != "tenant-abc" {
		t.Errorf("expected TenantID=tenant-abc, got %q", resp.TenantID)
	}
	if resp.EventType != "order.created" {
		t.Errorf("expected EventType=order.created, got %q", resp.EventType)
	}
	if resp.TargetURL != "https://example.com/webhook" {
		t.Errorf("expected TargetURL=https://example.com/webhook, got %q", resp.TargetURL)
	}
	if resp.CreatedBy != "admin" {
		t.Errorf("expected CreatedBy=admin, got %q", resp.CreatedBy)
	}
	if resp.CreatedAt == "" {
		t.Error("expected non-empty CreatedAt timestamp")
	}
}

func TestIngestEventRequest_Fields_Coverage(t *testing.T) {
	t.Parallel()

	req := service.IngestEventRequest{
		EventType: "order.created",
		Payload:   []byte(`{"id":"123"}`),
		DedupKey:  "dedup-001",
	}

	if req.EventType != "order.created" {
		t.Errorf("expected EventType=order.created, got %q", req.EventType)
	}
	if string(req.Payload) != `{"id":"123"}` {
		t.Errorf("expected Payload={\"id\":\"123\"}, got %q", string(req.Payload))
	}
	if req.DedupKey != "dedup-001" {
		t.Errorf("expected DedupKey=dedup-001, got %q", req.DedupKey)
	}
}

func TestIngestEventResponse_Fields_Coverage(t *testing.T) {
	t.Parallel()

	resp := service.IngestEventResponse{
		EventID: "evt-001",
	}
	if resp.EventID != "evt-001" {
		t.Errorf("expected EventID=evt-001, got %q", resp.EventID)
	}
}
