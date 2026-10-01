package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
)

type inMemSubRepo struct {
	mu   sync.Mutex
	byID map[string]*domain.Subscription
}

func newInMemSubRepo() *inMemSubRepo { return &inMemSubRepo{byID: map[string]*domain.Subscription{}} }
func (r *inMemSubRepo) Create(_ context.Context, s *domain.Subscription) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[s.ID] = s
	return s, nil
}
func (r *inMemSubRepo) FindByID(_ context.Context, tid, id string) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok || s.TenantID != tid {
		return nil, domain.ErrNotFound
	}
	return s, nil
}
func (r *inMemSubRepo) ListByTenant(_ context.Context, tid string) ([]domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Subscription
	for _, s := range r.byID {
		if s.TenantID == tid {
			out = append(out, *s)
		}
	}
	return out, nil
}
func (r *inMemSubRepo) Delete(_ context.Context, tid, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok || s.TenantID != tid {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

func TestSubRepoCRUD(t *testing.T) {
	t.Parallel()
	repo := newInMemSubRepo()
	ctx := context.Background()
	s := &domain.Subscription{ID: "s1", TenantID: "t1", EventType: "order.created", TargetURL: "https://example.com/webhook"}
	if created, err := repo.Create(ctx, s); err != nil || created.ID != "s1" {
		t.Fatal("create failed")
	}
	found, err := repo.FindByID(ctx, "t1", "s1")
	if err != nil || found.EventType != "order.created" {
		t.Fatal("find failed")
	}
}

func TestSubRepoCrossTenant(t *testing.T) {
	t.Parallel()
	repo := newInMemSubRepo()
	ctx := context.Background()
	repo.Create(ctx, &domain.Subscription{ID: "s1", TenantID: "t1", EventType: "x", TargetURL: "http://x"})
	if _, err := repo.FindByID(ctx, "t2", "s1"); err != domain.ErrNotFound {
		t.Fatalf("cross-tenant read must return ErrNotFound, got %v", err)
	}
	if err := repo.Delete(ctx, "t2", "s1"); err != domain.ErrNotFound {
		t.Fatalf("cross-tenant delete must return ErrNotFound, got %v", err)
	}
	if list, _ := repo.ListByTenant(ctx, "t2"); len(list) != 0 {
		t.Fatal("t2 should see empty list")
	}
}
