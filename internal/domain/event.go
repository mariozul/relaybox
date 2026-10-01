package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event represents an ingested webhook event to be relayed to subscribers.
type Event struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	DedupKey  string          `json:"dedup_key"`

	// Audit columns (RULE-DATA-03)
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// NewEvent creates a validated Event entity.
// Returns ErrInvalidInput if event_type is empty or payload is nil.
func NewEvent(tenantID, eventType string, payload json.RawMessage, dedupKey, createdBy string) (*Event, error) {
	if eventType == "" {
		return nil, fmt.Errorf("%w: event_type must not be empty", ErrInvalidInput)
	}
	if payload == nil {
		return nil, fmt.Errorf("%w: payload must not be nil", ErrInvalidInput)
	}
	return &Event{
		TenantID:  tenantID,
		EventType: eventType,
		Payload:   payload,
		DedupKey:  dedupKey,
		CreatedBy: createdBy,
	}, nil
}
