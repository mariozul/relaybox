package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
)

type inMemEventRepo struct {
	mu      sync.Mutex
	byID    map[string]*domain.Event
	byDedup map[string]*domain.Event
}

func newInMemEventRepo() *inMemEventRepo {
	return &inMemEventRepo{byID: map[string]*domain.Event{}, byDedup: map[string]*domain.Event{}}
}
func (r *inMemEventRepo) Create(_ context.Context, e *domain.Event) (*domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ex, ok := r.byDedup[e.TenantID+"/"+e.DedupKey]; ok {
		return ex, domain.ErrConflict
	}
	r.byID[e.ID] = e
	r.byDedup[e.TenantID+"/"+e.DedupKey] = e
	return e, nil
}
func (r *inMemEventRepo) FindByDedup(_ context.Context, tid, dk string) (*domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.byDedup[tid+"/"+dk]; ok {
		return e, nil
	}
	return nil, domain.ErrNotFound
}
func (r *inMemEventRepo) FindByID(_ context.Context, id string) (*domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.byID[id]; ok {
		return e, nil
	}
	return nil, domain.ErrNotFound
}

func TestEventRepoIdempotent(t *testing.T) {
	t.Parallel()
	repo := newInMemEventRepo()
	e1 := &domain.Event{ID: "ev-1", TenantID: "t1", EventType: "x", Payload: []byte("p"), DedupKey: "dk"}
	if _, err := repo.Create(context.Background(), e1); err != nil {
		t.Fatal(err)
	}
	e2 := &domain.Event{ID: "ev-2", TenantID: "t1", EventType: "x", Payload: []byte("p"), DedupKey: "dk"}
	r, err := repo.Create(context.Background(), e2)
	if err != domain.ErrConflict || r.ID != "ev-1" {
		t.Fatalf("expected ErrConflict+ev-1, got %v/%s", err, r.ID)
	}
}

func TestEventRepoCrossTenant(t *testing.T) {
	t.Parallel()
	repo := newInMemEventRepo()
	repo.Create(context.Background(), &domain.Event{ID: "e1", TenantID: "t1", EventType: "x", Payload: []byte("p"), DedupKey: "dk"})
	created, err := repo.Create(context.Background(), &domain.Event{ID: "e2", TenantID: "t2", EventType: "x", Payload: []byte("p"), DedupKey: "dk"})
	if err != nil || created.ID != "e2" {
		t.Fatalf("cross-tenant same dedup should succeed: %v", err)
	}
	_, err = repo.FindByDedup(context.Background(), "t1", "unknown")
	if err != domain.ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
}
