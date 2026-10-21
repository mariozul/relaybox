package repository_test

import (
	"context"
	"testing"
	"time"

	_ "github.com/lib/pq"
	repoimpl "github.com/mariozul/relaybox/pkg/relaybox/infrastructure/repository"
)

// ============================================================
// TC-SUB-01-INT: Subscription CRUD operations (FR-SUB-01)
// ============================================================
func TestPostgresSubscriptionRepository_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	subRepo := repoimpl.NewPostgresSubscriptionRepository(db)
	ctx := context.Background()

	sub := makeSubscription("tenant-alpha")

	id, err := subRepo.CreateSubscription(ctx, sub)
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}
	if id != sub.ID {
		t.Errorf("id: got %s, want %s", id, sub.ID)
	}

	found, err := subRepo.GetSubscriptionByID(ctx, sub.TenantID, sub.ID)
	if err != nil {
		t.Fatalf("GetSubscriptionByID: %v", err)
	}
	if found == nil {
		t.Fatal("expected non-nil subscription")
	}
	if found.ID != sub.ID {
		t.Errorf("id: got %s, want %s", found.ID, sub.ID)
	}
	if found.TargetURL != sub.TargetURL {
		t.Errorf("target_url: got %s, want %s", found.TargetURL, sub.TargetURL)
	}
}

func TestPostgresSubscriptionRepository_List(t *testing.T) {
	db := setupTestDB(t)
	subRepo := repoimpl.NewPostgresSubscriptionRepository(db)
	ctx := context.Background()

	subA := makeSubscription("tenant-alpha")
	subA.TargetURL = "https://alpha.example.com/webhook"
	subB := makeSubscription("tenant-beta")
	subB.TargetURL = "https://beta.example.com/webhook"

	_, err := subRepo.CreateSubscription(ctx, subA)
	if err != nil {
		t.Fatalf("Create subA: %v", err)
	}
	_, err = subRepo.CreateSubscription(ctx, subB)
	if err != nil {
		t.Fatalf("Create subB: %v", err)
	}

	subs, err := subRepo.ListSubscriptions(ctx, "tenant-alpha")
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 for tenant-alpha, got %d", len(subs))
	}
	if subs[0].ID != subA.ID {
		t.Errorf("expected subA id, got %s", subs[0].ID)
	}
}

func TestPostgresSubscriptionRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	subRepo := repoimpl.NewPostgresSubscriptionRepository(db)
	ctx := context.Background()

	sub := makeSubscription("tenant-alpha")
	_, err := subRepo.CreateSubscription(ctx, sub)
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	sub.TargetURL = "https://updated.example.com/webhook"
	sub.UpdatedBy = "updater"
	sub.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

	err = subRepo.UpdateSubscription(ctx, sub)
	if err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}

	found, err := subRepo.GetSubscriptionByID(ctx, sub.TenantID, sub.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if found.TargetURL != "https://updated.example.com/webhook" {
		t.Errorf("target_url: got %s, want updated", found.TargetURL)
	}
	if found.UpdatedBy != "updater" {
		t.Errorf("updated_by: got %s, want updater", found.UpdatedBy)
	}
}

func TestPostgresSubscriptionRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	subRepo := repoimpl.NewPostgresSubscriptionRepository(db)
	ctx := context.Background()

	sub := makeSubscription("tenant-alpha")
	_, err := subRepo.CreateSubscription(ctx, sub)
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	err = subRepo.DeleteSubscription(ctx, sub.TenantID, sub.ID)
	if err != nil {
		t.Fatalf("DeleteSubscription: %v", err)
	}

	found, err := subRepo.GetSubscriptionByID(ctx, sub.TenantID, sub.ID)
	if err != nil || found != nil {
		t.Errorf("expected nil after delete, got %v (err: %v)", found, err)
	}
}

// ============================================================
// TC-SUB-02-INT: Tenant isolation (FR-SUB-02, AC-04)
// ============================================================
func TestPostgresSubscriptionRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	subRepo := repoimpl.NewPostgresSubscriptionRepository(db)
	ctx := context.Background()

	subA := makeSubscription("tenant-a")
	subA.EventType = "order.created"
	subB := makeSubscription("tenant-b")
	subB.EventType = "order.updated"

	_, err := subRepo.CreateSubscription(ctx, subA)
	if err != nil {
		t.Fatalf("Create subA: %v", err)
	}
	_, err = subRepo.CreateSubscription(ctx, subB)
	if err != nil {
		t.Fatalf("Create subB: %v", err)
	}

	// tenant-a lists - should only see subA
	subs, err := subRepo.ListSubscriptions(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("List tenant-a: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 for tenant-a, got %d", len(subs))
	}
	if subs[0].ID == subB.ID {
		t.Error("tenant-a should not see tenant-b subscription")
	}

	// tenant-a gets subB by ID - should return nil
	found, err := subRepo.GetSubscriptionByID(ctx, "tenant-a", subB.ID)
	if err != nil || found != nil {
		t.Errorf("tenant-a getting subB should return nil, got %v (err: %v)", found, err)
	}

	// tenant-b lists - should only see subB
	subs, err = subRepo.ListSubscriptions(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("List tenant-b: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 for tenant-b, got %d", len(subs))
	}
	if subs[0].ID != subB.ID {
		t.Error("tenant-b should see their own subscription")
	}
}
