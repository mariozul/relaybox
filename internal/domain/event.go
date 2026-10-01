package domain

import "time"

// Event represents an ingested webhook event scoped to a tenant.
// Audit columns (RULE-DATA-03) are tracked for every mutation.
type Event struct {
	ID        string
	TenantID  string
	EventType string
	Payload   []byte
	DedupKey  string
	// Audit columns (RULE-DATA-03).
	CreatedBy string
	CreatedAt time.Time
	UpdatedBy string
	UpdatedAt time.Time
}

// NewEvent creates a valid Event with audit metadata.
func NewEvent(tenantID, eventType string, payload []byte, dedupKey, createdBy string, now time.Time) Event {
	return Event{
		TenantID:  tenantID,
		EventType: eventType,
		Payload:   payload,
		DedupKey:  dedupKey,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedBy: createdBy,
		UpdatedAt: now,
	}
}

// Validate checks basic business invariants on the event.
func (e Event) Validate() error {
	if e.TenantID == "" {
		return ErrInvalidInput
	}
	if e.EventType == "" {
		return ErrInvalidInput
	}
	if len(e.Payload) == 0 {
		return ErrInvalidInput
	}
	if e.DedupKey == "" {
		return ErrInvalidInput
	}
	return nil
}
