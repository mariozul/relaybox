package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/dto"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// ============================================================
// TC-SUB-01-INT: CRUD operations for subscriptions (FR-SUB-01)
// ============================================================

// fakeSubRepo implements repository.SubscriptionRepository for testing.
type fakeSubRepo struct {
	subs map[string]*domain.Subscription // key: id
}

func newFakeSubRepo() *fakeSubRepo {
	return &fakeSubRepo{subs: make(map[string]*domain.Subscription)}
}

func (f *fakeSubRepo) CreateSubscription(ctx context.Context, sub *domain.Subscription) (string, error) {
	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	f.subs[sub.ID] = sub
	return sub.ID, nil
}

func (f *fakeSubRepo) GetSubscriptionByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	sub, ok := f.subs[id]
	if !ok || sub.TenantID != tenantID {
		return nil, nil
	}
	return sub, nil
}

func (f *fakeSubRepo) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	var result []domain.Subscription
	for _, sub := range f.subs {
		if sub.TenantID == tenantID {
			result = append(result, *sub)
		}
	}
	if result == nil {
		result = []domain.Subscription{}
	}
	return result, nil
}

func (f *fakeSubRepo) UpdateSubscription(ctx context.Context, sub *domain.Subscription) error {
	existing, ok := f.subs[sub.ID]
	if !ok || existing.TenantID != sub.TenantID {
		return domain.ErrSubscriptionNotFound
	}
	f.subs[sub.ID] = sub
	return nil
}

func (f *fakeSubRepo) DeleteSubscription(ctx context.Context, tenantID, id string) error {
	sub, ok := f.subs[id]
	if !ok || sub.TenantID != tenantID {
		return domain.ErrSubscriptionNotFound
	}
	delete(f.subs, id)
	return nil
}

func fixedClock() time.Time {
	return time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
}

func TestSubscriptionService_CreateAndGet(t *testing.T) {
	// Phase RED: This test will initially fail - we implement the service to make it pass.
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	tenantID := "tenant-alpha"
	req := &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://hooks.example.com/order-created",
	}

	id, err := svc.CreateSubscription(ctx, tenantID, req)
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty subscription ID")
	}

	// Retrieve by ID
	sub, err := svc.GetSubscription(ctx, tenantID, id)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if sub == nil {
		t.Fatal("expected non-nil subscription")
	}
	if sub.EventType != req.EventType {
		t.Errorf("EventType: got %s, want %s", sub.EventType, req.EventType)
	}
	if sub.TargetURL != req.TargetURL {
		t.Errorf("TargetURL: got %s, want %s", sub.TargetURL, req.TargetURL)
	}
	if sub.TenantID != tenantID {
		t.Errorf("TenantID: got %s, want %s", sub.TenantID, tenantID)
	}
}

func TestSubscriptionService_List(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	// Create two subscriptions for tenant-alpha
	_, err := svc.CreateSubscription(ctx, "tenant-alpha", &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://alpha.example.com/webhook1",
	})
	if err != nil {
		t.Fatalf("Create sub1: %v", err)
	}

	_, err = svc.CreateSubscription(ctx, "tenant-alpha", &dto.CreateSubscriptionRequest{
		EventType: "order.updated",
		TargetURL: "https://alpha.example.com/webhook2",
	})
	if err != nil {
		t.Fatalf("Create sub2: %v", err)
	}

	// Create subscription for tenant-beta
	_, err = svc.CreateSubscription(ctx, "tenant-beta", &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://beta.example.com/webhook",
	})
	if err != nil {
		t.Fatalf("Create sub3: %v", err)
	}

	// List for tenant-alpha
	subs, err := svc.ListSubscriptions(ctx, "tenant-alpha")
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions for tenant-alpha, got %d", len(subs))
	}
}

func TestSubscriptionService_Update(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	tenantID := "tenant-alpha"
	id, err := svc.CreateSubscription(ctx, tenantID, &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	})
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	newURL := "https://updated.example.com/webhook"
	err = svc.UpdateSubscription(ctx, tenantID, id, &dto.UpdateSubscriptionRequest{
		TargetURL: &newURL,
	})
	if err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}

	sub, err := svc.GetSubscription(ctx, tenantID, id)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if sub.TargetURL != newURL {
		t.Errorf("TargetURL: got %s, want %s", sub.TargetURL, newURL)
	}
}

func TestSubscriptionService_Delete(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	tenantID := "tenant-alpha"
	id, err := svc.CreateSubscription(ctx, tenantID, &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://example.com/webhook",
	})
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	err = svc.DeleteSubscription(ctx, tenantID, id)
	if err != nil {
		t.Fatalf("DeleteSubscription: %v", err)
	}

	// Should not be found after deletion
	_, err = svc.GetSubscription(ctx, tenantID, id)
	if err != domain.ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound after delete, got %v", err)
	}
}

// ============================================================
// TC-SUB-02-INT: Tenant isolation (FR-SUB-02, AC-04, RULE-SEC-01)
// ============================================================

func TestSubscriptionService_TenantIsolation_Get(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	// TenantA creates a subscription
	idA, err := svc.CreateSubscription(ctx, "tenant-a", &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://a.example.com/webhook",
	})
	if err != nil {
		t.Fatalf("Create tenant-a sub: %v", err)
	}

	// TenantB creates a subscription
	_, err = svc.CreateSubscription(ctx, "tenant-b", &dto.CreateSubscriptionRequest{
		EventType: "order.updated",
		TargetURL: "https://b.example.com/webhook",
	})
	if err != nil {
		t.Fatalf("Create tenant-b sub: %v", err)
	}

	// TenantA should NOT be able to fetch TenantB's subscription (AC-04)
	_, err = svc.GetSubscription(ctx, "tenant-a", idA)
	if err != nil {
		t.Fatalf("TenantA should get own sub, got: %v", err)
	}

	// Actually the original test was checking cross-tenant access
	// Let's fix: Get the ID of TenantB's subscription by listing
	tenantBSubs, err := svc.ListSubscriptions(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("List tenant-b: %v", err)
	}
	if len(tenantBSubs) != 1 {
		t.Fatalf("expected 1 sub for tenant-b, got %d", len(tenantBSubs))
	}
	idB := tenantBSubs[0].ID

	// TenantA should not be able to access TenantB's subscription
	sub, err := svc.GetSubscription(ctx, "tenant-a", idB)
	if err == nil || sub != nil {
		t.Errorf("TenantA should NOT access TenantB's sub: got sub=%v err=%v", sub, err)
	}
	if err != domain.ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionService_TenantIsolation_List(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	// Create subscriptions for different tenants
	_, err := svc.CreateSubscription(ctx, "tenant-a", &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://a.example.com/webhook",
	})
	if err != nil {
		t.Fatalf("Create tenant-a sub: %v", err)
	}

	_, err = svc.CreateSubscription(ctx, "tenant-b", &dto.CreateSubscriptionRequest{
		EventType: "order.updated",
		TargetURL: "https://b.example.com/webhook",
	})
	if err != nil {
		t.Fatalf("Create tenant-b sub: %v", err)
	}

	// TenantA lists - should only see their own
	subs, err := svc.ListSubscriptions(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("List tenant-a: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 for tenant-a, got %d", len(subs))
	}
	if subs[0].TenantID != "tenant-a" {
		t.Errorf("sub belongs to %s, not tenant-a", subs[0].TenantID)
	}
}

func TestSubscriptionService_TenantIsolation_Update(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	// TenantA creates a subscription
	idA, err := svc.CreateSubscription(ctx, "tenant-a", &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://a.example.com/webhook",
	})
	if err != nil {
		t.Fatalf("Create tenant-a sub: %v", err)
	}

	// TenantB should NOT be able to update TenantA's subscription
	newURL := "https://malicious.example.com/webhook"
	err = svc.UpdateSubscription(ctx, "tenant-b", idA, &dto.UpdateSubscriptionRequest{
		TargetURL: &newURL,
	})
	if err != domain.ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound for cross-tenant update, got %v", err)
	}

	// Verify TenantA's subscription was NOT modified
	sub, err := svc.GetSubscription(ctx, "tenant-a", idA)
	if err != nil {
		t.Fatalf("Get tenant-a sub: %v", err)
	}
	if sub.TargetURL == newURL {
		t.Error("TenantB should not have been able to modify TenantA's subscription")
	}
}

func TestSubscriptionService_TenantIsolation_Delete(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	// TenantA creates a subscription
	idA, err := svc.CreateSubscription(ctx, "tenant-a", &dto.CreateSubscriptionRequest{
		EventType: "order.created",
		TargetURL: "https://a.example.com/webhook",
	})
	if err != nil {
		t.Fatalf("Create tenant-a sub: %v", err)
	}

	// TenantB should NOT be able to delete TenantA's subscription
	err = svc.DeleteSubscription(ctx, "tenant-b", idA)
	if err != domain.ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound for cross-tenant delete, got %v", err)
	}

	// Verify TenantA's subscription still exists
	sub, err := svc.GetSubscription(ctx, "tenant-a", idA)
	if err != nil {
		t.Fatalf("TenantA sub should still exist, got: %v", err)
	}
	if sub == nil {
		t.Fatal("TenantA sub should not be nil after cross-tenant delete attempt")
	}
}

func TestSubscriptionService_Validation(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	t.Run("empty tenant returns ErrUnauthorized", func(t *testing.T) {
		_, err := svc.CreateSubscription(ctx, "", &dto.CreateSubscriptionRequest{
			EventType: "order.created",
			TargetURL: "https://example.com/webhook",
		})
		if err != domain.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty event type returns ErrInvalidInput", func(t *testing.T) {
		_, err := svc.CreateSubscription(ctx, "tenant-a", &dto.CreateSubscriptionRequest{
			EventType: "",
			TargetURL: "https://example.com/webhook",
		})
		if err != domain.ErrInvalidInput {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("empty target URL returns ErrInvalidInput", func(t *testing.T) {
		_, err := svc.CreateSubscription(ctx, "tenant-a", &dto.CreateSubscriptionRequest{
			EventType: "order.created",
			TargetURL: "",
		})
		if err != domain.ErrInvalidInput {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("get non-existent returns ErrSubscriptionNotFound", func(t *testing.T) {
		_, err := svc.GetSubscription(ctx, "tenant-a", "non-existent-id")
		if err != domain.ErrSubscriptionNotFound {
			t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
		}
	})
}

func TestSubscriptionService_EmptyList(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	// Tenant with no subscriptions should get an empty list
	subs, err := svc.ListSubscriptions(ctx, "tenant-with-no-subs")
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	if subs == nil {
		t.Fatal("expected empty slice, not nil")
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(subs))
	}
}

func TestSubscriptionService_GetValidation(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	t.Run("empty tenant returns ErrUnauthorized", func(t *testing.T) {
		_, err := svc.GetSubscription(ctx, "", "some-id")
		if err != domain.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty id returns ErrInvalidInput", func(t *testing.T) {
		_, err := svc.GetSubscription(ctx, "tenant-a", "")
		if err != domain.ErrInvalidInput {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestSubscriptionService_ListValidation(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	t.Run("empty tenant returns ErrUnauthorized", func(t *testing.T) {
		_, err := svc.ListSubscriptions(ctx, "")
		if err != domain.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestSubscriptionService_UpdateValidation(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	t.Run("empty tenant returns ErrUnauthorized", func(t *testing.T) {
		newURL := "https://example.com/webhook"
		err := svc.UpdateSubscription(ctx, "", "some-id", &dto.UpdateSubscriptionRequest{
			TargetURL: &newURL,
		})
		if err != domain.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty id returns ErrInvalidInput", func(t *testing.T) {
		newURL := "https://example.com/webhook"
		err := svc.UpdateSubscription(ctx, "tenant-a", "", &dto.UpdateSubscriptionRequest{
			TargetURL: &newURL,
		})
		if err != domain.ErrInvalidInput {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("nil fields returns ErrInvalidInput", func(t *testing.T) {
		err := svc.UpdateSubscription(ctx, "tenant-a", "some-id", &dto.UpdateSubscriptionRequest{})
		if err != domain.ErrInvalidInput {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("update non-existent returns ErrSubscriptionNotFound", func(t *testing.T) {
		newURL := "https://example.com/webhook"
		err := svc.UpdateSubscription(ctx, "tenant-a", "non-existent-id", &dto.UpdateSubscriptionRequest{
			TargetURL: &newURL,
		})
		if err != domain.ErrSubscriptionNotFound {
			t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
		}
	})
}

func TestSubscriptionService_DeleteValidation(t *testing.T) {
	fakeRepo := newFakeSubRepo()
	svc := service.NewSubscriptionService(fakeRepo, fixedClock)
	ctx := context.Background()

	t.Run("empty tenant returns ErrUnauthorized", func(t *testing.T) {
		err := svc.DeleteSubscription(ctx, "", "some-id")
		if err != domain.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("empty id returns ErrInvalidInput", func(t *testing.T) {
		err := svc.DeleteSubscription(ctx, "tenant-a", "")
		if err != domain.ErrInvalidInput {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("delete non-existent returns ErrSubscriptionNotFound", func(t *testing.T) {
		err := svc.DeleteSubscription(ctx, "tenant-a", "non-existent-id")
		if err != domain.ErrSubscriptionNotFound {
			t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
		}
	})
}
