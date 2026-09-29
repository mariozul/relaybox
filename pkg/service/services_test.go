package service

import (
	"context"
	"errors"
	"testing"

	"github.com/mariozul/relaybox/pkg/domain"
)

type fakeStore struct {
	check    func(context.Context)
	identity domain.Identity
	calls    int
}

func (f *fakeStore) Ingest(ctx context.Context, i domain.Identity, _ domain.EventInput) (domain.Event, error) {
	f.check(ctx)
	f.identity = i
	f.calls++
	return domain.Event{ID: "event"}, nil
}
func (f *fakeStore) CreateSubscription(ctx context.Context, i domain.Identity, _ domain.SubscriptionInput) (domain.Subscription, error) {
	f.check(ctx)
	f.identity = i
	f.calls++
	return domain.Subscription{ID: "sub"}, nil
}
func (f *fakeStore) ListSubscriptions(ctx context.Context, i domain.Identity) ([]domain.Subscription, error) {
	f.check(ctx)
	f.identity = i
	f.calls++
	return []domain.Subscription{{ID: "sub"}}, nil
}
func (f *fakeStore) DeleteSubscription(ctx context.Context, i domain.Identity, _ string) error {
	f.check(ctx)
	f.identity = i
	f.calls++
	return nil
}

func TestServices(t *testing.T) {
	ctx := t.Context()
	f := &fakeStore{check: func(got context.Context) {
		if got != ctx {
			t.Fatal("lost context")
		}
	}}
	events := Events{Store: f}
	subscriptions := Subscriptions{Store: f}
	identity := domain.Identity{TenantID: "tenant", ActorID: "actor"}
	for _, valid := range []bool{true, false} {
		i := identity
		if !valid {
			i = domain.Identity{}
		}
		for _, call := range []func() error{
			func() error { _, err := events.Ingest(ctx, i, domain.EventInput{}); return err },
			func() error { _, err := subscriptions.Create(ctx, i, domain.SubscriptionInput{}); return err },
			func() error { _, err := subscriptions.List(ctx, i); return err },
			func() error { return subscriptions.Delete(ctx, i, "sub") },
		} {
			before := f.calls
			err := call()
			if valid {
				if err != nil || f.calls != before+1 || f.identity != identity {
					t.Fatalf("lost context/identity: %v", err)
				}
			} else if !errors.Is(err, domain.ErrUnauthorized) || f.calls != before {
				t.Fatalf("invalid principal reached store: %v", err)
			}
		}
	}
}
