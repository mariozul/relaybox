package dispatcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestProcessRetriesAndDeadletters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		status, attempts int
		sendErr          error
		dead             bool
	}{
		{"success", 204, 0, nil, false}, {"server error", 500, 0, nil, false}, {"client max", 400, 2, nil, true}, {"network max", 0, 2, errors.New("down"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{delivery: domain.Delivery{ID: "delivery", Attempts: tc.attempts}}
			d := New(store, fakeSender{tc.status, tc.sendErr}, Config{MaxAttempts: 3, RequestTimeout: time.Second})
			d.process(context.Background(), store.delivery)
			if store.dead != tc.dead {
				t.Fatalf("dead=%v", store.dead)
			}
		})
	}
}

type fakeSender struct {
	status int
	err    error
}

func (f fakeSender) Send(context.Context, domain.Delivery) (int, error) { return f.status, f.err }

type fakeStore struct {
	delivery domain.Delivery
	dead     bool
}

func (f *fakeStore) Ingest(context.Context, domain.Event) (domain.Event, bool, error) {
	return domain.Event{}, false, nil
}
func (f *fakeStore) CreateSubscription(context.Context, domain.Subscription) (domain.Subscription, error) {
	return domain.Subscription{}, nil
}
func (f *fakeStore) ListSubscriptions(context.Context) ([]domain.Subscription, error) {
	return nil, nil
}
func (f *fakeStore) DeleteSubscription(context.Context, string) error { return nil }
func (f *fakeStore) Claim(context.Context, time.Time, int, time.Duration) ([]domain.Delivery, error) {
	return nil, nil
}
func (f *fakeStore) MarkDelivered(context.Context, domain.Delivery, time.Time) error { return nil }
func (f *fakeStore) MarkFailed(_ context.Context, _ domain.Delivery, _ time.Time, _ string, dead bool) error {
	f.dead = dead
	return nil
}
func (f *fakeStore) Ping(context.Context) error { return nil }
