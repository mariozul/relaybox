package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

type inMemOutboxRepo struct {
	mu      sync.Mutex
	entries map[string]*domain.OutboxEntry
}

func newInMemOutboxRepo() *inMemOutboxRepo { return &inMemOutboxRepo{entries: map[string]*domain.OutboxEntry{}} }
func (r *inMemOutboxRepo) Create(_ context.Context, e *domain.OutboxEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[e.ID] = e
	return nil
}
func (r *inMemOutboxRepo) ClaimPending(_ context.Context, now time.Time, limit int) ([]domain.OutboxEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.OutboxEntry
	for _, e := range r.entries {
		if e.Status == domain.OutboxStatusPending && !e.NextAttemptAt.After(now) && len(out) < limit {
			out = append(out, *e)
		}
	}
	return out, nil
}
func (r *inMemOutboxRepo) MarkDelivered(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.OutboxStatusDelivered
	return nil
}
func (r *inMemOutboxRepo) MarkFailed(_ context.Context, id string, attempts int, next time.Time, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Attempts = attempts
	e.NextAttemptAt = next
	e.LastError = lastErr
	e.Status = domain.OutboxStatusFailed
	return nil
}
func (r *inMemOutboxRepo) MarkDeadLetter(_ context.Context, id, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.OutboxStatusDeadLetter
	e.LastError = lastErr
	return nil
}

func TestOutboxLifecycle(t *testing.T) {
	t.Parallel()
	repo := newInMemOutboxRepo()
	ctx := context.Background()
	now := time.Now()

	repo.Create(ctx, &domain.OutboxEntry{ID: "o1", Status: domain.OutboxStatusPending, NextAttemptAt: now})
	claimed, _ := repo.ClaimPending(ctx, now.Add(time.Second), 10)
	if len(claimed) != 1 || claimed[0].ID != "o1" {
		t.Fatal("claim failed")
	}
	repo.MarkDelivered(ctx, "o1")
	claimed2, _ := repo.ClaimPending(ctx, now.Add(time.Hour), 10)
	if len(claimed2) != 0 {
		t.Fatal("delivered should not be claimable")
	}
	repo.Create(ctx, &domain.OutboxEntry{ID: "o2", Status: domain.OutboxStatusPending, NextAttemptAt: now})
	repo.MarkFailed(ctx, "o2", 1, now.Add(5*time.Second), "timeout")
	claimed3, _ := repo.ClaimPending(ctx, now, 10)
	if len(claimed3) != 0 {
		t.Fatal("failed entry with future next should not be claimable")
	}
	repo.Create(ctx, &domain.OutboxEntry{ID: "o3", Status: domain.OutboxStatusPending, NextAttemptAt: now})
	repo.MarkDeadLetter(ctx, "o3", "400")
	claimed4, _ := repo.ClaimPending(ctx, now.Add(time.Hour), 10)
	if len(claimed4) != 0 {
		t.Fatal("dead letter should not be claimable")
	}
}
