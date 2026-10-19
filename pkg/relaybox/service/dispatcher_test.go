package service_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/infrastructure/httpclient"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
	"github.com/mariozul/relaybox/pkg/relaybox/worker"
)

// --- Dispatcher-specific mocks (avoid conflicting with ingestion_test.go) ---

type dispSubRepo struct {
	sync.Mutex
	subs map[string]*domain.Subscription
}

func newDispSubRepo() *dispSubRepo {
	return &dispSubRepo{subs: make(map[string]*domain.Subscription)}
}

func (m *dispSubRepo) add(t *testing.T, s *domain.Subscription) {
	t.Helper()
	m.Lock()
	defer m.Unlock()
	m.subs[s.ID] = s
}

func (m *dispSubRepo) GetSubscriptionByID(_ context.Context, tenantID, id string) (*domain.Subscription, error) {
	m.Lock()
	defer m.Unlock()
	s, ok := m.subs[id]
	if !ok || (tenantID != "" && s.TenantID != tenantID) {
		return nil, nil
	}
	return s, nil
}

func (m *dispSubRepo) CreateSubscription(_ context.Context, s *domain.Subscription) (string, error) {
	m.Lock()
	defer m.Unlock()
	m.subs[s.ID] = s
	return s.ID, nil
}
func (m *dispSubRepo) ListSubscriptions(_ context.Context, _ string) ([]domain.Subscription, error) { return nil, nil }
func (m *dispSubRepo) UpdateSubscription(_ context.Context, _ *domain.Subscription) error { return nil }
func (m *dispSubRepo) DeleteSubscription(_ context.Context, _, _ string) error { return nil }

type dispOutboxRepo struct {
	sync.Mutex
	entries map[string]*domain.OutboxEntry
}

func newDispOutboxRepo() *dispOutboxRepo {
	return &dispOutboxRepo{entries: make(map[string]*domain.OutboxEntry)}
}

func (m *dispOutboxRepo) add(t *testing.T, e *domain.OutboxEntry) {
	t.Helper()
	m.Lock()
	defer m.Unlock()
	m.entries[e.ID] = e
}

func (m *dispOutboxRepo) EnqueueEvent(_ context.Context, e *domain.OutboxEntry) (string, error) {
	m.Lock()
	defer m.Unlock()
	m.entries[e.ID] = e
	return e.ID, nil
}

func (m *dispOutboxRepo) GetPendingOutboxEntries(_ context.Context, limit int) ([]domain.OutboxEntry, error) {
	m.Lock()
	defer m.Unlock()
	now := time.Now()
	var pending []domain.OutboxEntry
	for _, e := range m.entries {
		if (e.Status == "pending" || e.Status == "failed") && !e.NextAttemptAt.After(now) {
			pending = append(pending, *e)
		}
	}
	if len(pending) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}

func (m *dispOutboxRepo) MarkOutboxEntryProcessed(_ context.Context, id, status string, attempts int, next *time.Time, errStr *string) error {
	m.Lock()
	defer m.Unlock()
	if e, ok := m.entries[id]; ok {
		e.Status = status
		e.Attempts = attempts
		if next != nil {
			e.NextAttemptAt = *next
		}
		if errStr != nil {
			e.LastAttemptError = *errStr
		}
	}
	return nil
}

// --- Tests ---

func runDispatcher(t *testing.T, outboxRepo *dispOutboxRepo, subRepo *dispSubRepo, timeout time.Duration) {
	t.Helper()
	hc := httpclient.NewDefaultHTTPClient(5 * time.Second)
	pool := worker.NewWorkerPool(context.Background(), 5, 20)
	disp := service.NewDispatcher(outboxRepo, subRepo, hc, pool, testClock)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		disp.Start(ctx)
	}()
	wg.Wait()
}

func makeOB(id, subID, deliveryID string) *domain.OutboxEntry {
	return &domain.OutboxEntry{
		ID: id, EventID: "evt-" + id, SubscriptionID: subID,
		Status: "pending", Attempts: 0,
		NextAttemptAt: testClock().Add(-1 * time.Minute),
		DeliveryID: deliveryID,
		CreatedAt: testClock().Add(-10 * time.Minute),
	}
}

func TestDispatcher(t *testing.T) {
	t.Parallel()

	t.Run("delivers pending events on start", func(t *testing.T) {
		t.Parallel()
		var delivered int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&delivered, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, &domain.Subscription{ID: "s1", TenantID: "t1",
			TargetURL: srv.URL, CreatedAt: testClock(), CreatedBy: "t",
			UpdatedAt: testClock(), UpdatedBy: "t"})
		ob := newDispOutboxRepo()
		ob.add(t, makeOB("ob1", "s1", "d1"))

		runDispatcher(t, ob, sub, 2*time.Second)
		if atomic.LoadInt32(&delivered) == 0 {
			t.Error("expected at least one delivery")
		}
	})

	t.Run("retries 5xx then succeeds", func(t *testing.T) {
		t.Parallel()
		var attempt atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempt.Add(1) < 3 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, &domain.Subscription{ID: "s2", TenantID: "t2",
			TargetURL: srv.URL, CreatedAt: testClock(), CreatedBy: "t",
			UpdatedAt: testClock(), UpdatedBy: "t"})
		ob := newDispOutboxRepo()
		ob.add(t, makeOB("ob2", "s2", "d2"))

		runDispatcher(t, ob, sub, 5*time.Second)
		if attempt.Load() < 3 {
			t.Errorf("expected >=3 attempts, got %d", attempt.Load())
		}
	})

	t.Run("dead-letters 4xx after max attempts", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, &domain.Subscription{ID: "s3", TenantID: "t3",
			TargetURL: srv.URL, CreatedAt: testClock(), CreatedBy: "t",
			UpdatedAt: testClock(), UpdatedBy: "t"})
		ob := newDispOutboxRepo()
		ob.add(t, makeOB("ob3", "s3", "d3"))

		runDispatcher(t, ob, sub, 5*time.Second)

		ob.Lock()
		e := ob.entries["ob3"]
		ob.Unlock()
		if e.Status != "dead_letter" {
			t.Errorf("expected dead_letter, got %q (attempts=%d)", e.Status, e.Attempts)
		}
		if e.Attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", e.Attempts)
		}
	})

	t.Run("sends X-Relaybox-Delivery-Id header", func(t *testing.T) {
		t.Parallel()
		ch := make(chan string, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ch <- r.Header.Get("X-Relaybox-Delivery-Id")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, &domain.Subscription{ID: "s4", TenantID: "t4",
			TargetURL: srv.URL, CreatedAt: testClock(), CreatedBy: "t",
			UpdatedAt: testClock(), UpdatedBy: "t"})
		ob := newDispOutboxRepo()
		ob.add(t, makeOB("ob4", "s4", "del-abc"))

		runDispatcher(t, ob, sub, 2*time.Second)
		select {
		case h := <-ch:
			if h != "del-abc" {
				t.Errorf("expected del-abc, got %q", h)
			}
		default:
			t.Error("no delivery captured")
		}
	})

	t.Run("handles concurrent load", func(t *testing.T) {
		t.Parallel()
		var delivered int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&delivered, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, &domain.Subscription{ID: "s5", TenantID: "t5",
			TargetURL: srv.URL, CreatedAt: testClock(), CreatedBy: "t",
			UpdatedAt: testClock(), UpdatedBy: "t"})
		ob := newDispOutboxRepo()
		for i := 0; i < 20; i++ {
			ob.add(t, makeOB(fmt.Sprintf("oc-%02d", i), "s5", fmt.Sprintf("dc-%02d", i)))
		}

		runDispatcher(t, ob, sub, 3*time.Second)
		if d := atomic.LoadInt32(&delivered); d == 0 {
			t.Error("expected at least some deliveries")
		} else {
			t.Logf("delivered %d events", d)
		}
	})
}
