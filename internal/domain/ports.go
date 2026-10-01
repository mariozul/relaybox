package domain

import "context"

// EventRepository defines persistence operations for Event entities.
// All methods accept context.Context per RULE-RES-01.
type EventRepository interface {
	// Create persists a new event. Returns ErrDuplicateEvent on duplicate dedup_key.
	Create(ctx context.Context, event *Event) error

	// GetByDedupKey retrieves an existing event by tenant and dedup key
	// for idempotent ingestion (FR-ING-03).
	GetByDedupKey(ctx context.Context, tenantID, dedupKey string) (*Event, error)

	// GetByID retrieves an event by its primary key.
	GetByID(ctx context.Context, id string) (*Event, error)
}

// SubscriptionRepository defines persistence operations for Subscription entities.
type SubscriptionRepository interface {
	// Create persists a new subscription scoped to the given tenant.
	Create(ctx context.Context, sub *Subscription) error

	// ListByTenant returns all subscriptions for a tenant.
	ListByTenant(ctx context.Context, tenantID string) ([]Subscription, error)

	// ListByEventType returns subscriptions matching an event type
	// across all tenants (for dispatch lookups).
	ListByEventType(ctx context.Context, eventType string) ([]Subscription, error)

	// Delete removes a subscription by ID, scoped to the owning tenant.
	Delete(ctx context.Context, tenantID, id string) error
}

// OutboxRepository defines persistence operations for OutboxEntry entities.
type OutboxRepository interface {
	// Create persists a new outbox delivery entry.
	Create(ctx context.Context, entry *OutboxEntry) error

	// FetchPending retrieves outbox rows due for delivery
	// (status IN ('pending','failed') AND next_attempt_at <= now) with bounded batch size.
	FetchPending(ctx context.Context, limit int) ([]OutboxEntry, error)

	// UpdateStatus persists a status change for an outbox row.
	UpdateStatus(ctx context.Context, entry *OutboxEntry) error
}

// DeliveryGateway defines the outbound HTTP delivery contract for dispatching
// webhook events to subscriber endpoints (FR-DEL-01).
type DeliveryGateway interface {
	// Send delivers the event payload to a subscriber target URL.
	// Returns nil on successful 2xx delivery, or an error on failure.
	// Must propagate ctx and set a stable X-Relaybox-Delivery-Id header (FR-DEL-04, RULE-RES-01).
	Send(ctx context.Context, deliveryID, targetURL string, payload []byte) error
}
