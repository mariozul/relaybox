package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/domain"
)

func TestClaims(t *testing.T) {
	ctx, tx := fixture(t)
	store := New(tx, fixedClock{})
	owner := domain.Identity{TenantID: "a", ActorID: "actor"}
	if _, err := store.CreateSubscription(ctx, owner, domain.SubscriptionInput{EventType: "order", TargetURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(ctx, owner, domain.EventInput{EventType: "order", DedupKey: "key", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	claims, err := store.Claim(ctx, 1, fixedClock{}.Now().Add(time.Minute))
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim %v %v", claims, err)
	}
	first := claims[0]
	if first.Attempts != 1 || first.LeaseToken == "" || string(first.Payload) != "{}" {
		t.Fatalf("bad claim: %+v", first)
	}
	claims, err = store.Claim(ctx, 1, fixedClock{}.Now().Add(time.Minute))
	if err != nil || len(claims) != 0 {
		t.Fatalf("double claim %v %v", claims, err)
	}
	if _, err := tx.Exec(ctx, "UPDATE outbox SET lease_until=$1", fixedClock{}.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	claims, err = store.Claim(ctx, 1, fixedClock{}.Now().Add(time.Minute))
	if err != nil || len(claims) != 1 {
		t.Fatalf("reclaim %v %v", claims, err)
	}
	if claims[0].ID != first.ID || claims[0].LeaseToken == first.LeaseToken || claims[0].Attempts != 2 {
		t.Fatal("unstable ID or unfenced reclaim")
	}
	if err := store.Complete(ctx, first, domain.Outcome{State: domain.Delivered}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale completion %v", err)
	}
	if err := store.Complete(ctx, claims[0], domain.Outcome{State: domain.Delivered}); err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, claims[0], domain.Outcome{State: domain.Pending}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("terminal changed %v", err)
	}
	if _, err := store.Claim(ctx, 0, fixedClock{}.Now()); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid claim %v", err)
	}
	if err := store.Complete(ctx, first, domain.Outcome{State: domain.Processing}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid outcome %v", err)
	}
}
