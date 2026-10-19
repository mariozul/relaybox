package repository

import (
	"context"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// EventRepository defines contract for event persistence.
type EventRepository interface {
	// CreateEvent inserts new event.
	// Returns ErrEventAlreadyExists if dedup_key already exists tenant.
	CreateEvent(ctx context.Context, event *domain.Event) (string, error)

	// GetEventByDedupKey retrieves an event by its tenant and dedup key.
	GetEventByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error)

	// GetEventByID retrieves an event by its ID.
	// Returns nil, nil if not found.
	GetEventByID(ctx context.Context, id string) (*domain.Event, error)
}
