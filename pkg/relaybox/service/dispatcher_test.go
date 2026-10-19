package service_test

import (
	"context"
	"encoding/json"
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

// dispEventRepo is a mock EventRepository for dispatcher tests.
type dispEventRepo struct {
	sync.Mutex
	events map[string]*domain.Event
}

func newDispEventRepo() *dispEventRepo {
	return &dispEventRepo{events: make(map[string]*domain.Event)}
}

func (m *dispEventRepo) add(t *testing.T, e *domain.Event) {
	t.Helper()
	m.Lock()
	defer m.Unlock()
	m.events[e.ID] = e
}

func (m *dispEventRepo) CreateEvent(_ context.Context, e *domain.Event) (string, error) {
	m.Lock()
	defer m.Unlock()
	m.events[e.ID] = e
	return e.ID, nil
}

func (m *dispEventRepo) GetEventByDedupKey(_ context.Context, _, _ string) (*domain.Event, error) {
	return nil, nil
}

func (m *dispEventRepo) GetEventByID(_ context.Context, id string) (*domain.Event, error) {
	m.Lock()
	defer m.Unlock()
	e, ok := m.events[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}

// Helper to create a test event linked to an outbox entry.
func makeEventForOB(entry *domain.OutboxEntry) *domain.Event {
	return &domain.Event{
		ID:        entry.EventID,
		TenantID:  "t1",
		EventType: "test.event",
		Payload:   []byte(`{"key":"value"}`),
		DedupKey:  "dedup-" + entry.ID,
		CreatedAt: testClock(),
		CreatedBy: "test",
	}
}

// --- Tests ---

func runDispatcher(t *testing.T, obRepo *dispOutboxRepo, subRepo *dispSubRepo, evtRepo *dispEventRepo, timeout time.Duration) {
	t.Helper()
	hc := httpclient.NewDefaultHTTPClient(5 * time.Second)
	pool := worker.NewWorkerPool(context.Background(), 5, 20)
	disp := service.NewDispatcher(obRepo, subRepo, evtRepo, hc, pool, testClock)

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
		DeliveryID:    deliveryID,
		CreatedAt:     testClock().Add(-10 * time.Minute),
	}
}

func makeSub(t *testing.T, id, tenantID, targetURL string) *domain.Subscription {
	t.Helper()
	return &domain.Subscription{
		ID: id, TenantID: tenantID, EventType: "test.event",
		TargetURL: targetURL,
		CreatedAt: testClock(), CreatedBy: tenantID,
		UpdatedAt: testClock(), UpdatedBy: tenantID,
	}
}

func TestDispatcher(t *testing.T) {
	t.Parallel()

	t.Run("delivers pending events on start", func(t *testing.T) {
		t.Parallel()
		var (
			delivered    int32
			receivedBody string
			mu           sync.Mutex
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&delivered, 1)
			buf := make([]byte, r.ContentLength)
			r.Body.Read(buf)
			r.Body.Close()
			mu.Lock()
			receivedBody = string(buf)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, makeSub(t, "s1", "t1", srv.URL))

		ob := newDispOutboxRepo()
		obEntry := makeOB("ob1", "s1", "d1")
		ob.add(t, obEntry)

		evtRepo := newDispEventRepo()
		evtRepo.add(t, makeEventForOB(obEntry))

		runDispatcher(t, ob, sub, evtRepo, 2*time.Second)

		if atomic.LoadInt32(&delivered) == 0 {
			t.Error("expected at least one delivery")
		}

		// Verify the delivery body contains expected fields.
		mu.Lock()
		body := receivedBody
		mu.Unlock()
		if body != "" {
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(body), &payload); err != nil {
				t.Fatalf("failed to unmarshal delivery body: %v", err)
			}
			if payload["event_id"] != "evt-ob1" {
				t.Errorf("expected event_id evt-ob1, got %v", payload["event_id"])
			}
			if payload["delivery_id"] != "d1" {
				t.Errorf("expected delivery_id d1, got %v", payload["delivery_id"])
			}
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
		sub.add(t, makeSub(t, "s2", "t2", srv.URL))

		ob := newDispOutboxRepo()
		obEntry := makeOB("ob2", "s2", "d2")
		ob.add(t, obEntry)

		evtRepo := newDispEventRepo()
		evtRepo.add(t, makeEventForOB(obEntry))

		runDispatcher(t, ob, sub, evtRepo, 6*time.Second)

		if attempt.Load() < 3 {
			t.Errorf("expected >=3 attempts, got %d", attempt.Load())
		}
	})

	t.Run("dead-letters after max attempts", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, makeSub(t, "s3", "t3", srv.URL))

		ob := newDispOutboxRepo()
		obEntry := makeOB("ob3", "s3", "d3")
		ob.add(t, obEntry)

		evtRepo := newDispEventRepo()
		evtRepo.add(t, makeEventForOB(obEntry))

		runDispatcher(t, ob, sub, evtRepo, 6*time.Second)

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
		sub.add(t, makeSub(t, "s4", "t4", srv.URL))

		ob := newDispOutboxRepo()
		obEntry := makeOB("ob4", "s4", "del-abc")
		ob.add(t, obEntry)

		evtRepo := newDispEventRepo()
		evtRepo.add(t, makeEventForOB(obEntry))

		runDispatcher(t, ob, sub, evtRepo, 2*time.Second)

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
		sub.add(t, makeSub(t, "s5", "t5", srv.URL))

		ob := newDispOutboxRepo()
		evtRepo := newDispEventRepo()
		for i := 0; i < 20; i++ {
			obEntry := makeOB(fmt.Sprintf("oc-%02d", i), "s5", fmt.Sprintf("dc-%02d", i))
			ob.add(t, obEntry)
			evtRepo.add(t, makeEventForOB(obEntry))
		}

		runDispatcher(t, ob, sub, evtRepo, 3*time.Second)
		if d := atomic.LoadInt32(&delivered); d == 0 {
			t.Error("expected at least some deliveries")
		} else {
			t.Logf("delivered %d events", d)
		}
	})

	t.Run("dead-letters on missing event", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sub := newDispSubRepo()
		sub.add(t, makeSub(t, "s6", "t6", srv.URL))

		ob := newDispOutboxRepo()
		obEntry := makeOB("ob6", "s6", "d6")
		ob.add(t, obEntry)

		// Event repository is empty — event not found.
		evtRepo := newDispEventRepo()

		runDispatcher(t, ob, sub, evtRepo, 2*time.Second)

		ob.Lock()
		e := ob.entries["ob6"]
		ob.Unlock()
		if e.Status != "dead_letter" {
			t.Errorf("expected dead_letter for missing event, got %q", e.Status)
		}
	})
}
