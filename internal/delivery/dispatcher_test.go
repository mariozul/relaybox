package delivery

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

type mockOutboxRepoForDispatch struct {
	mu sync.Mutex
}

func (r *mockOutboxRepoForDispatch) CreateBatch(ctx context.Context, msgs []domain.OutboxMessage) error { return nil }
func (r *mockOutboxRepoForDispatch) ClaimPending(ctx context.Context, limit int) ([]domain.OutboxMessage, error) {
	return []domain.OutboxMessage{}, nil
}
func (r *mockOutboxRepoForDispatch) MarkDelivered(ctx context.Context, id string) error { return nil }
func (r *mockOutboxRepoForDispatch) MarkRetry(ctx context.Context, id string, nextAttemptAt time.Time) error {
	return nil
}
func (r *mockOutboxRepoForDispatch) DeadLetter(ctx context.Context, id string) error { return nil }

type mockEventRepoForDispatch struct{}

func (r *mockEventRepoForDispatch) CreateOrGet(ctx context.Context, event domain.Event) (*domain.Event, error) {
	return &domain.Event{ID: "evt-1", Payload: []byte(`{"x":1}`)}, nil
}

type mockSubRepoForDispatch struct{}

func (r *mockSubRepoForDispatch) Create(ctx context.Context, sub domain.Subscription) (*domain.Subscription, error) {
	return nil, nil
}
func (r *mockSubRepoForDispatch) GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	return nil, nil
}
func (r *mockSubRepoForDispatch) ListByTenant(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return nil, nil
}
func (r *mockSubRepoForDispatch) ListByEventType(ctx context.Context, tenantID, eventType string) ([]domain.Subscription, error) {
	return nil, nil
}
func (r *mockSubRepoForDispatch) Delete(ctx context.Context, tenantID, id string) error { return nil }

func TestDispatcher_BoundedPoolShutdown(t *testing.T) {
	t.Parallel()
	cfg := DefaultDispatcherConfig()
	cfg.WorkerCount = 2
	d := NewDispatcher(cfg, &mockOutboxRepoForDispatch{}, &mockEventRepoForDispatch{}, &mockSubRepoForDispatch{}, NewHTTPSender(DefaultSenderConfig()))
	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	d.Stop()
}
