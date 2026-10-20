package service_test

import (
	"context"
	"errors"
	"testing"


	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// ============================================================================
// Mock Subscription Repository for service-level tests
// ============================================================================

// mockSubscriptionRepository implements repository.SubscriptionRepository for
// testing SubscriptionService operations without a real database.
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
	// Check context cancellation (RULE-RES-01).
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	m.subs[sub.ID] = sub
	return sub, nil
}

func (m *mockSubscriptionRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}
	// Check context cancellation.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
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
	// Check context cancellation.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
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
	// Check context cancellation.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
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

// ============================================================================
// TC-04-UNIT: SubscriptionService CRUD operations
// ============================================================================

func TestSubscriptionService_CreateSubscription_Success(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}

	sub, err := svc.CreateSubscription(ctx, "tenant-abc", req)
	if err != nil {
		t.Fatalf("CreateSubscription() returned error: %v", err)
	}

	if sub == nil {
		t.Fatal("CreateSubscription() returned nil subscription")
	}
	if sub.TenantID != "tenant-abc" {
		t.Errorf("expected TenantID=%q, got %q", "tenant-abc", sub.TenantID)
	}
	if sub.EventType != "order.created" {
		t.Errorf("expected EventType=%q, got %q", "order.created", sub.EventType)
	}
	if sub.TargetURL != "https://example.com/webhook" {
		t.Errorf("expected TargetURL=%q, got %q", "https://example.com/webhook", sub.TargetURL)
	}
	if sub.CreatedBy != "admin" {
		t.Errorf("expected CreatedBy=%q, got %q", "admin", sub.CreatedBy)
	}
	if sub.ID == "" {
		t.Error("expected non-empty subscription ID")
	}
	if sub.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt timestamp")
	}
}

func TestSubscriptionService_CreateSubscription_EmptyEventType(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	req := service.CreateSubscriptionRequest{
		EventType: "",
		TargetURL: "https://example.com/webhook",
	}

	_, err := svc.CreateSubscription(ctx, "tenant-abc", req)
	if err == nil {
		t.Fatal("CreateSubscription() expected error for empty EventType, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidEventPayload) {
		t.Errorf("expected ErrInvalidEventPayload, got %v", err)
	}
}

func TestSubscriptionService_CreateSubscription_InvalidTargetURL(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "not-a-valid-url",
	}

	_, err := svc.CreateSubscription(ctx, "tenant-abc", req)
	if err == nil {
		t.Fatal("CreateSubscription() expected error for invalid URL, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidEventPayload) {
		t.Errorf("expected ErrInvalidEventPayload, got %v", err)
	}
}

func TestSubscriptionService_CreateSubscription_EmptyTenant(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}

	_, err := svc.CreateSubscription(ctx, "", req)
	if err == nil {
		t.Fatal("CreateSubscription() expected error for empty tenant, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidEventPayload) {
		t.Errorf("expected ErrInvalidEventPayload, got %v", err)
	}
}

func TestSubscriptionService_GetSubscription_Success(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	// Create a subscription first.
	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}
	created, err := svc.CreateSubscription(ctx, "tenant-abc", req)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// Retrieve by ID.
	got, err := svc.GetSubscription(ctx, "tenant-abc", created.ID)
	if err != nil {
		t.Fatalf("GetSubscription() returned error: %v", err)
	}
	if got == nil {
		t.Fatal("GetSubscription() returned nil")
	}
	if got.ID != created.ID {
		t.Errorf("expected ID=%q, got %q", created.ID, got.ID)
	}
	if got.TenantID != "tenant-abc" {
		t.Errorf("expected TenantID=%q, got %q", "tenant-abc", got.TenantID)
	}
}

func TestSubscriptionService_GetSubscription_NotFound(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	_, err := svc.GetSubscription(ctx, "tenant-abc", "nonexistent")
	if err == nil {
		t.Fatal("GetSubscription() expected error for nonexistent ID, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionService_ListSubscriptions_Success(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	// Create two subscriptions for tenant-abc.
	req1 := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook1",
	}
	req2 := service.CreateSubscriptionRequest{
		EventType: "order.shipped",
		TargetURL: "https://example.com/webhook2",
	}

	_, err := svc.CreateSubscription(ctx, "tenant-abc", req1)
	if err != nil {
		t.Fatalf("failed to create subscription 1: %v", err)
	}
	_, err = svc.CreateSubscription(ctx, "tenant-abc", req2)
	if err != nil {
		t.Fatalf("failed to create subscription 2: %v", err)
	}

	// Create a subscription for a different tenant (tenant-xyz).
	req3 := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://other.com/webhook",
	}
	_, err = svc.CreateSubscription(ctx, "tenant-xyz", req3)
	if err != nil {
		t.Fatalf("failed to create subscription for tenant-xyz: %v", err)
	}

	// List for tenant-abc should return only their two subscriptions.
	subs, err := svc.ListSubscriptions(ctx, "tenant-abc")
	if err != nil {
		t.Fatalf("ListSubscriptions() returned error: %v", err)
	}
	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions for tenant-abc, got %d", len(subs))
	}
}

func TestSubscriptionService_ListSubscriptions_Empty(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	subs, err := svc.ListSubscriptions(ctx, "tenant-abc")
	if err != nil {
		t.Fatalf("ListSubscriptions() returned error: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected empty list, got %d subscription(s)", len(subs))
	}
}

func TestSubscriptionService_DeleteSubscription_Success(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}
	created, err := svc.CreateSubscription(ctx, "tenant-abc", req)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	err = svc.DeleteSubscription(ctx, "tenant-abc", created.ID)
	if err != nil {
		t.Fatalf("DeleteSubscription() returned error: %v", err)
	}

	// Verify it's deleted.
	_, err = svc.GetSubscription(ctx, "tenant-abc", created.ID)
	if err == nil {
		t.Fatal("expected subscription to be deleted, but GetSubscription returned no error")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionService_DeleteSubscription_NotFound(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	err := svc.DeleteSubscription(ctx, "tenant-abc", "nonexistent")
	if err == nil {
		t.Fatal("DeleteSubscription() expected error for nonexistent ID, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// ============================================================================
// TC-05-INT: Tenant isolation for subscriptions
// Tenant A cannot access or modify Tenant B's subscriptions (FR-SUB-02,
// RULE-SEC-01).
// ============================================================================

func TestSubscriptionService_TenantIsolation_GetCrossTenant(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	// Tenant A creates a subscription.
	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://tenant-a.com/webhook",
	}
	tenantASub, err := svc.CreateSubscription(ctx, "tenant-a", req)
	if err != nil {
		t.Fatalf("failed to create subscription for tenant-a: %v", err)
	}

	// Tenant B should NOT be able to retrieve Tenant A's subscription.
	_, err = svc.GetSubscription(ctx, "tenant-b", tenantASub.ID)
	if err == nil {
		t.Fatal("GetSubscription() expected error when tenant-b accesses tenant-a's subscription, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound for cross-tenant access, got %v", err)
	}
}

func TestSubscriptionService_TenantIsolation_DeleteCrossTenant(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	// Tenant A creates a subscription.
	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://tenant-a.com/webhook",
	}
	tenantASub, err := svc.CreateSubscription(ctx, "tenant-a", req)
	if err != nil {
		t.Fatalf("failed to create subscription for tenant-a: %v", err)
	}

	// Tenant B should NOT be able to delete Tenant A's subscription.
	err = svc.DeleteSubscription(ctx, "tenant-b", tenantASub.ID)
	if err == nil {
		t.Fatal("DeleteSubscription() expected error when tenant-b deletes tenant-a's subscription, got nil")
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Errorf("expected ErrSubscriptionNotFound for cross-tenant deletion, got %v", err)
	}

	// Tenant A's subscription should still exist.
	got, err := svc.GetSubscription(ctx, "tenant-a", tenantASub.ID)
	if err != nil {
		t.Fatalf("tenant-a's subscription should still exist after tenant-b attempted deletion: %v", err)
	}
	if got == nil {
		t.Fatal("tenant-a's subscription should still exist")
	}
}

func TestSubscriptionService_TenantIsolation_ListIsScoped(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	// Tenant A creates 2 subscriptions.
	reqA1 := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://tenant-a.com/webhook1",
	}
	reqA2 := service.CreateSubscriptionRequest{
		EventType: "order.shipped",
		TargetURL: "https://tenant-a.com/webhook2",
	}
	_, err := svc.CreateSubscription(ctx, "tenant-a", reqA1)
	if err != nil {
		t.Fatalf("failed to create subscription for tenant-a: %v", err)
	}
	_, err = svc.CreateSubscription(ctx, "tenant-a", reqA2)
	if err != nil {
		t.Fatalf("failed to create subscription for tenant-a: %v", err)
	}

	// Tenant B creates 1 subscription.
	reqB := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://tenant-b.com/webhook",
	}
	_, err = svc.CreateSubscription(ctx, "tenant-b", reqB)
	if err != nil {
		t.Fatalf("failed to create subscription for tenant-b: %v", err)
	}

	// Tenant A should see exactly 2 subscriptions.
	tenantASubs, err := svc.ListSubscriptions(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("ListSubscriptions() for tenant-a returned error: %v", err)
	}
	if len(tenantASubs) != 2 {
		t.Errorf("expected 2 subscriptions for tenant-a, got %d", len(tenantASubs))
	}

	// Tenant B should see exactly 1 subscription.
	tenantBSubs, err := svc.ListSubscriptions(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("ListSubscriptions() for tenant-b returned error: %v", err)
	}
	if len(tenantBSubs) != 1 {
		t.Errorf("expected 1 subscription for tenant-b, got %d", len(tenantBSubs))
	}
}

func TestSubscriptionService_TenantIsolation_EmptyTenant(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)
	ctx := context.Background()

	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}

	// Creating with empty tenant should fail.
	_, err := svc.CreateSubscription(ctx, "", req)
	if err == nil {
		t.Fatal("CreateSubscription() expected error for empty tenant ID, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidEventPayload) {
		t.Errorf("expected ErrInvalidEventPayload, got %v", err)
	}
}

// ============================================================================
// Context propagation test (RULE-RES-01): Service must accept and propagate
// context to repository calls.
// ============================================================================

func TestSubscriptionService_ContextCancellation(t *testing.T) {
	t.Parallel()

	repo := newMockSubscriptionRepository()
	svc := service.NewSubscriptionService(repo)

	// Create a cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := service.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	}

	// Should fail due to cancelled context.
	_, err := svc.CreateSubscription(ctx, "tenant-abc", req)
	if err == nil {
		t.Fatal("CreateSubscription() expected error for cancelled context, got nil")
	}
}
