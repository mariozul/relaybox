package repository

import (
	"context"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// EventRepository defines the contract for event persistence.
type EventRepository interface {
	// CreateEvent inserts a new event.
	// Returns ErrEventAlreadyExists if dedup_key already exists for tenant.
	CreateEvent(ctx context.Context, event *domain.Event) (string, error)

	// GetEventByDedupKey retrieves an event by its tenant and dedup key.
	GetEventByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error)
}
