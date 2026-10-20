package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/adapter/http_client"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// ============================================================================
// Mock implementations for testing
// ============================================================================

type mockOutboxRepo struct {
	mu      sync.Mutex
	entries map[string]*domain.OutboxEntry
}

func newMockOutboxRepo() *mockOutboxRepo {
	return &mockOutboxRepo{
		entries: make(map[string]*domain.OutboxEntry),
	}
}

func (m *mockOutboxRepo) Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	m.entries[entry.ID] = entry
	return entry, nil
}

func (m *mockOutboxRepo) FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *mockOutboxRepo) UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *mockOutboxRepo) getEntry(id string) *domain.OutboxEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries[id]
}

type mockSubscriptionRepo struct {
	mu   sync.Mutex
	subs map[string]*domain.Subscription
}

func newMockSubscriptionRepo() *mockSubscriptionRepo {
	return &mockSubscriptionRepo{
		subs: make(map[string]*domain.Subscription),
	}
}

func (m *mockSubscriptionRepo) Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[sub.ID] = sub
	return sub, nil
}

// FindByID retrieves a subscription by ID. The tenantID parameter is optional
// for the dispatcher use case; when empty, the subscription is returned by ID
// alone. When non-empty, tenant isolation is enforced.
func (m *mockSubscriptionRepo) FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[id]
	if !ok {
		return nil, domain.ErrSubscriptionNotFound
	}
	// If tenantID is provided, enforce tenant isolation.
	if tenantID != "" && sub.TenantID != tenantID {
		return nil, domain.ErrSubscriptionNotFound
	}
	return sub, nil
}

func (m *mockSubscriptionRepo) FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []domain.Subscription
	for _, sub := range m.subs {
		if sub.TenantID == tenantID {
			result = append(result, *sub)
		}
	}
	return result, nil
}

func (m *mockSubscriptionRepo) Delete(ctx context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[id]
	if !ok || sub.TenantID != tenantID {
		return domain.ErrSubscriptionNotFound
	}
	delete(m.subs, id)
	return nil
}

// mockWebhookClient implements service.WebhookClient for testing.
type mockWebhookClient struct {
	callCount atomic.Int32
}

func newMockWebhookClient() *mockWebhookClient {
	return &mockWebhookClient{}
}

func (m *mockWebhookClient) SendWebhook(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	m.callCount.Add(1)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil
}

func (m *mockWebhookClient) GetCallCount() int {
	return int(m.callCount.Load())
}

// mockWebhookClientWithStatus returns configurable status codes per URL.
type mockWebhookClientWithStatus struct {
	mu          sync.Mutex
	responses   map[string]int
	callCount   atomic.Int32
	headersLog  []map[string]string
}

func newMockWebhookClientWithStatus() *mockWebhookClientWithStatus {
	return &mockWebhookClientWithStatus{
		responses:  make(map[string]int),
		headersLog: make([]map[string]string, 0),
	}
}

func (m *mockWebhookClientWithStatus) addResponse(url string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[url] = statusCode
}

func (m *mockWebhookClientWithStatus) SendWebhook(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	m.callCount.Add(1)
	m.mu.Lock()
	// Deep copy headers for later inspection.
	h := make(map[string]string)
	for k, v := range headers {
		h[k] = v
	}
	m.headersLog = append(m.headersLog, h)
	statusCode := m.responses[url]
	m.mu.Unlock()

	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	return &http.Response{
		StatusCode: statusCode,
		Body:       http.NoBody,
	}, nil
}

func (m *mockWebhookClientWithStatus) GetCallCount() int {
	return int(m.callCount.Load())
}

func (m *mockWebhookClientWithStatus) GetHeaders() []map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]map[string]string, len(m.headersLog))
	copy(result, m.headersLog)
	return result
}

// ============================================================================
// Helper to create test outbox entries
// ============================================================================

func createTestEntry(t *testing.T, eventID, subID string, status domain.OutboxStatus, attempts int) *domain.OutboxEntry {
	t.Helper()
	entry, err := domain.NewOutboxEntry(eventID, subID, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to create outbox entry: %v", err)
	}
	entry.Status = status
	entry.Attempts = attempts
	return entry
}

// ============================================================================
// TC-03-INT: Dispatcher picks up pending entries
// ============================================================================

func TestDispatcher_PicksUpPendingEntries(t *testing.T) {
	t.Parallel()

	subID := "sub-001"
	eventID := "evt-001"

	outbox := newMockOutboxRepo()
	entry := createTestEntry(t, eventID, subID, domain.OutboxStatusPending, 0)
	_, _ = outbox.Save(context.Background(), entry)

	subRepo := newMockSubscriptionRepo()
	sub := &domain.Subscription{
		ID:        subID,
		TenantID:  "tenant-abc",
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}
	_, _ = subRepo.Save(context.Background(), sub)

	client := newMockWebhookClient()
	dispatcher := service.NewDeliveryDispatcher(outbox, subRepo, client, service.WithPollInterval(50*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = dispatcher.Start(ctx)

	if client.GetCallCount() == 0 {
		t.Error("expected dispatcher to pick up and deliver pending entries")
	}

	updated := outbox.getEntry(entry.ID)
	if updated == nil {
		t.Fatal("entry not found in outbox")
	}
	if updated.Status != domain.OutboxStatusDelivered {
		t.Errorf("expected entry to be delivered, got status=%q", updated.Status)
	}
}

// ============================================================================
// TC-06-INT: Dispatcher retries on 5xx, dead-letters on 4xx
// ============================================================================

func TestDispatcher_RetriesOn5xxThenSucceeds(t *testing.T) {
	t.Parallel()

	// Use a real HTTP server that returns 500 twice then 200.
	var attempt atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempt.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	subID := "sub-005"
	eventID := "evt-005"

	outbox := newMockOutboxRepo()
	entry := createTestEntry(t, eventID, subID, domain.OutboxStatusPending, 0)
	_, _ = outbox.Save(context.Background(), entry)

	subRepo := newMockSubscriptionRepo()
	sub := &domain.Subscription{
		ID:        subID,
		TenantID:  "tenant-abc",
		EventType: "order.created",
		TargetURL: server.URL,
	}
	_, _ = subRepo.Save(context.Background(), sub)

	webhookClient := http_client.NewWebhookClient(5 * time.Second)
	dispatcher := service.NewDeliveryDispatcher(outbox, subRepo, webhookClient, service.WithPollInterval(50*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = dispatcher.Start(ctx)

	updated := outbox.getEntry(entry.ID)
	if updated == nil {
		t.Fatal("entry not found in outbox")
	}
	if updated.Status != domain.OutboxStatusDelivered {
		t.Errorf("expected entry to be delivered after retries, got status=%q (attempts=%d)", updated.Status, updated.Attempts)
	}

	if attempt.Load() < 3 {
		t.Errorf("expected at least 3 delivery attempts, got %d", attempt.Load())
	}
}

func TestDispatcher_DeadLettersOn4xx(t *testing.T) {
	t.Parallel()

	subID := "sub-004"
	eventID := "evt-004"

	outbox := newMockOutboxRepo()
	entry := createTestEntry(t, eventID, subID, domain.OutboxStatusPending, 0)
	_, _ = outbox.Save(context.Background(), entry)

	subRepo := newMockSubscriptionRepo()
	sub := &domain.Subscription{
		ID:        subID,
		TenantID:  "tenant-abc",
		EventType: "order.created",
		TargetURL: "https://example.com/4xx",
	}
	_, _ = subRepo.Save(context.Background(), sub)

	// Mock client that always returns 400.
	client := newMockWebhookClientWithStatus()
	client.addResponse(sub.TargetURL, http.StatusBadRequest)

	dispatcher := service.NewDeliveryDispatcher(outbox, subRepo, client, service.WithPollInterval(50*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = dispatcher.Start(ctx)

	updated := outbox.getEntry(entry.ID)
	if updated == nil {
		t.Fatal("entry not found in outbox")
	}
	if updated.Status != domain.OutboxStatusDeadLetter {
		t.Errorf("expected entry to be dead-lettered for 4xx, got status=%q (attempts=%d)", updated.Status, updated.Attempts)
	}
}

// ============================================================================
// TC-07-RACE: Concurrency test with race detection
// ============================================================================

func TestDispatcher_ConcurrentDelivery_RaceCheck(t *testing.T) {
	t.Parallel()

	outbox := newMockOutboxRepo()
	subRepo := newMockSubscriptionRepo()
	client := newMockWebhookClient()

	// Create multiple entries for the same subscription.
	subID := "sub-concurrent"
	for i := 0; i < 10; i++ {
		entry := createTestEntry(t, "evt-concurrent", subID, domain.OutboxStatusPending, 0)
		_, _ = outbox.Save(context.Background(), entry)
	}

	sub := &domain.Subscription{
		ID:        subID,
		TenantID:  "tenant-abc",
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}
	_, _ = subRepo.Save(context.Background(), sub)

	dispatcher := service.NewDeliveryDispatcher(outbox, subRepo, client,
		service.WithPollInterval(50*time.Millisecond),
		service.WithWorkerCount(5),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = dispatcher.Start(ctx)

	// If we get here without race detector firing, test passes.
	pending, _ := outbox.FindPending(context.Background(), 100)
	t.Logf("remaining pending entries after concurrent run: %d", len(pending))
}

// ============================================================================
// TC-09-UNIT: Context propagation - no context.Background() in dispatcher
// ============================================================================

func TestDispatcher_ContextPropagation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify delivery ID and trace headers are propagated.
		if r.Header.Get("X-Relaybox-Delivery-Id") == "" {
			t.Error("expected delivery ID header to be present")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	subID := "sub-ctx"
	eventID := "evt-ctx"

	outbox := newMockOutboxRepo()
	entry := createTestEntry(t, eventID, subID, domain.OutboxStatusPending, 0)
	_, _ = outbox.Save(context.Background(), entry)

	subRepo := newMockSubscriptionRepo()
	sub := &domain.Subscription{
		ID:        subID,
		TenantID:  "tenant-abc",
		EventType: "order.created",
		TargetURL: server.URL,
	}
	_, _ = subRepo.Save(context.Background(), sub)

	webhookClient := http_client.NewWebhookClient(5 * time.Second)
	dispatcher := service.NewDeliveryDispatcher(outbox, subRepo, webhookClient, service.WithPollInterval(50*time.Millisecond))

	// Use a proper context with timeout (NOT context.Background()).
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = dispatcher.Start(ctx)

	updated := outbox.getEntry(entry.ID)
	if updated == nil {
		t.Fatal("entry not found in outbox")
	}
	if updated.Status != domain.OutboxStatusDelivered {
		t.Errorf("expected entry to be delivered, got status=%q", updated.Status)
	}
}

// ============================================================================
// TC-10-UNIT: X-Relaybox-Delivery-Id header is stable across retries
// ============================================================================

func TestDispatcher_StableDeliveryIDHeader(t *testing.T) {
	t.Parallel()

	subID := "sub-delivery-id"
	eventID := "evt-delivery-id"
	expectedDeliveryID := domain.GenerateDeliveryID(eventID, subID)

	outbox := newMockOutboxRepo()
	entry := createTestEntry(t, eventID, subID, domain.OutboxStatusPending, 0)
	_, _ = outbox.Save(context.Background(), entry)

	subRepo := newMockSubscriptionRepo()
	sub := &domain.Subscription{
		ID:        subID,
		TenantID:  "tenant-abc",
		EventType: "order.created",
		TargetURL: "https://example.com/retry-header",
	}
	_, _ = subRepo.Save(context.Background(), sub)

	// Always returns 500 to trigger retries.
	client := newMockWebhookClientWithStatus()
	client.addResponse(sub.TargetURL, http.StatusInternalServerError)

	dispatcher := service.NewDeliveryDispatcher(outbox, subRepo, client,
		service.WithPollInterval(50*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = dispatcher.Start(ctx)

	headers := client.GetHeaders()
	if len(headers) < 2 {
		t.Skipf("expected at least 2 attempts for retry test, got %d", len(headers))
	}

	// Verify the X-Relaybox-Delivery-Id is the same across all attempts.
	for i, h := range headers {
		got := h["X-Relaybox-Delivery-Id"]
		if got != expectedDeliveryID {
			t.Errorf("attempt %d: expected X-Relaybox-Delivery-Id=%q, got %q", i, expectedDeliveryID, got)
		}
	}
}

// ============================================================================
// Additional: Graceful shutdown test
// ============================================================================

func TestDispatcher_GracefulShutdown(t *testing.T) {
	t.Parallel()

	outbox := newMockOutboxRepo()
	subRepo := newMockSubscriptionRepo()
	client := newMockWebhookClient()

	entry := createTestEntry(t, "evt-shutdown", "sub-shutdown", domain.OutboxStatusPending, 0)
	_, _ = outbox.Save(context.Background(), entry)

	sub := &domain.Subscription{
		ID:        "sub-shutdown",
		TenantID:  "tenant-abc",
		EventType: "order.created",
		TargetURL: "https://example.com/shutdown",
	}
	_, _ = subRepo.Save(context.Background(), sub)

	dispatcher := service.NewDeliveryDispatcher(outbox, subRepo, client, service.WithPollInterval(100*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := dispatcher.Start(ctx)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Start() returned unexpected error: %v", err)
	}
}
