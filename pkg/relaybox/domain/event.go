package domain

import (
	"time"

	"github.com/google/uuid"
)

// Event represents a single domain event ingested into Relaybox. It captures
// the event metadata, payload, and audit trail (RULE-DATA-03).
type Event struct {
	// ID is the unique identifier for this event (UUIDv7).
	ID string `json:"id" db:"id"`
	// TenantID is the tenant that owns this event.
	TenantID string `json:"tenant_id" db:"tenant_id"`
	// EventType is the domain event type (e.g., "order.created").
	EventType string `json:"event_type" db:"event_type"`
	// Payload is the raw JSON payload of the event.
	Payload []byte `json:"payload" db:"payload"`
	// DedupKey is the deduplication key for idempotent ingestion
	// (FR-ING-03, RULE-DATA-02).
	DedupKey string `json:"dedup_key" db:"dedup_key"`
	// CreatedAt is the UTC timestamp when the event was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// CreatedBy identifies the actor who created this event (RULE-DATA-03).
	CreatedBy string `json:"created_by" db:"created_by"`
}

// NewEvent creates a new Event with the given parameters. It validates that
// all required fields are present and returns ErrInvalidEventPayload if any
// required field is missing or invalid.
func NewEvent(tenantID, eventType string, payload []byte, dedupKey, createdBy string, createdAt time.Time) (*Event, error) {
	if tenantID == "" {
		return nil, ErrInvalidEventPayload
	}
	if eventType == "" {
		return nil, ErrInvalidEventPayload
	}
	if len(payload) == 0 {
		return nil, ErrInvalidEventPayload
	}
	if dedupKey == "" {
		return nil, ErrInvalidEventPayload
	}
	if createdBy == "" {
		return nil, ErrInvalidEventPayload
	}

	return &Event{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		EventType: eventType,
		Payload:   payload,
		DedupKey:  dedupKey,
		CreatedAt: createdAt,
		CreatedBy: createdBy,
	}, nil
}
