package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/domain"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }

func TestSubscriptions(t *testing.T) {
	ctx, tx := fixture(t)
	store := New(tx, fixedClock{})
	owner := domain.Identity{TenantID: "a", ActorID: "creator"}
	other := domain.Identity{TenantID: "b", ActorID: "other"}
	input := domain.SubscriptionInput{EventType: "order", TargetURL: "https://example.com/hook"}
	sub, err := store.CreateSubscription(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if sub.ID == "" || sub.TenantID != "a" || sub.Audit.CreatedBy != "creator" || !sub.Audit.CreatedAt.Equal(fixedClock{}.Now()) {
		t.Fatalf("bad subscription: %+v", sub)
	}
	for _, tc := range []struct {
		identity domain.Identity
		count    int
	}{{owner, 1}, {other, 0}} {
		got, err := store.ListSubscriptions(ctx, tc.identity)
		if err != nil || len(got) != tc.count {
			t.Fatalf("list: %v %v", got, err)
		}
	}
	if err := store.DeleteSubscription(ctx, other, sub.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	if err := store.DeleteSubscription(ctx, owner, sub.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListSubscriptions(ctx, owner)
	if err != nil || len(got) != 0 {
		t.Fatalf("deleted listed: %v %v", got, err)
	}
	var actor string
	if err := tx.QueryRow(ctx, "SELECT deleted_by FROM subscriptions WHERE id=$1", sub.ID).Scan(&actor); err != nil || actor != owner.ActorID {
		t.Fatalf("audit: %s %v", actor, err)
	}
	if err := store.DeleteSubscription(ctx, owner, sub.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("repeat delete: %v", err)
	}
	if _, err := store.CreateSubscription(ctx, domain.Identity{}, input); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("unauthorized: %v", err)
	}
	if _, err := store.ListSubscriptions(ctx, domain.Identity{}); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("unauthorized: %v", err)
	}
	if err := store.DeleteSubscription(ctx, domain.Identity{}, sub.ID); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("unauthorized: %v", err)
	}
	for _, bad := range []domain.SubscriptionInput{{EventType: "", TargetURL: input.TargetURL}, {EventType: "order", TargetURL: "http://example.com"}, {EventType: "order", TargetURL: "https://user:pass@example.com"}} {
		if _, err := store.CreateSubscription(ctx, owner, bad); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("invalid input: %v", err)
		}
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListSubscriptions(ctx, owner); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("closed tx: %v", err)
	}
}
