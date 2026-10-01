package service

import (
	"context"
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestSubscriptionService_CRUD(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	repo := newMockSubscriptionRepo()
	svc := NewSubscriptionService(repo, clock)
	ctx := context.Background()

	// Create.
	sub, err := svc.Create(ctx, CreateInput{
		TenantID:  "tenant-a",
		EventType: "order.created",
		TargetURL: "https://example.com/hook",
		CreatedBy: "system",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sub.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if sub.TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %s", sub.TenantID)
	}

	// GetByID.
	got, err := svc.GetByID(ctx, "tenant-a", sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != sub.ID {
		t.Fatalf("ID mismatch")
	}

	// List.
	list, err := svc.List(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(list))
	}

	// Delete.
	if err := svc.Delete(ctx, "tenant-a", sub.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Get after delete should return not found.
	_, err = svc.GetByID(ctx, "tenant-a", sub.ID)
	if !domain.IsNotFound(err) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSubscriptionService_CrossTenantIsolation(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	repo := newMockSubscriptionRepo()
	svc := NewSubscriptionService(repo, clock)
	ctx := context.Background()

	// Create subscription for tenant-a.
	subA, err := svc.Create(ctx, CreateInput{
		TenantID:  "tenant-a",
		EventType: "order.created",
		TargetURL: "https://a.example.com/hook",
		CreatedBy: "system",
	})
	if err != nil {
		t.Fatalf("create tenant-a: %v", err)
	}

	// Tenant-b tries to read tenant-a's subscription.
	_, err = svc.GetByID(ctx, "tenant-b", subA.ID)
	if !domain.IsNotFound(err) {
		t.Fatalf("expected ErrNotFound for cross-tenant access, got %v", err)
	}

	// Tenant-a list should show only tenant-a subs.
	listA, err := svc.List(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("list tenant-a: %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("tenant-a should have 1 sub, got %d", len(listA))
	}

	// Tenant-b list should be empty.
	listB, err := svc.List(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("list tenant-b: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("tenant-b should have 0 subs, got %d", len(listB))
	}

	// Tenant-b tries to delete tenant-a's subscription.
	err = svc.Delete(ctx, "tenant-b", subA.ID)
	if !domain.IsNotFound(err) {
		t.Fatalf("expected ErrNotFound for cross-tenant delete, got %v", err)
	}
}

func TestSubscriptionService_ValidateInput(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	svc := NewSubscriptionService(newMockSubscriptionRepo(), clock)

	_, err := svc.Create(context.Background(), CreateInput{
		TenantID:  "",
		EventType: "e1",
		TargetURL: "https://example.com/hook",
		CreatedBy: "s",
	})
	if !domain.IsInvalidInput(err) {
		t.Fatalf("expected ErrInvalidInput for missing tenant, got %v", err)
	}
}

func TestSubscriptionService_DuplicateCreate(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	repo := newMockSubscriptionRepo()
	svc := NewSubscriptionService(repo, clock)
	ctx := context.Background()
	input := CreateInput{
		TenantID:  "tenant-a",
		EventType: "order.created",
		TargetURL: "https://example.com/hook",
		CreatedBy: "system",
	}

	_, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = svc.Create(ctx, input)
	if !domain.IsConflict(err) {
		t.Fatalf("expected ErrConflict for duplicate, got %v", err)
	}
}
