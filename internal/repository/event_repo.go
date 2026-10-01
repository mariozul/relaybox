package repository

import (
	"context"

	"github.com/mariozul/relaybox/internal/domain"
)

// EventRepository defines persistence operations for domain Event entities.
type EventRepository interface {
	// Create inserts a new event. Must be idempotent on (tenant_id, dedup_key):
	// returns the existing event (with domain.ErrConflict) when a conflict is detected.
	Create(ctx context.Context, event *domain.Event) (*domain.Event, error)

	// FindByDedup returns the event matching (tenantID, dedupKey) or domain.ErrNotFound.
	FindByDedup(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error)

	// FindByID returns the event by primary key or domain.ErrNotFound.
	FindByID(ctx context.Context, id string) (*domain.Event, error)
}
