package domain

import (
	"time"
)

// EventStatus represents the lifecycle status of an ingested event.
type EventStatus string

const (
	EventStatusPending    EventStatus = "pending"
	EventStatusProcessing EventStatus = "processing"
	EventStatusCompleted  EventStatus = "completed"
	EventStatusFailed     EventStatus = "failed"
)

// Event represents a domain event ingested for relay to subscriber endpoints.
type Event struct {
	ID        string
	TenantID  string
	EventType string
	Payload   []byte
	DedupKey  string
	Status    EventStatus
	CreatedAt time.Time
	CreatedBy string
}

// Validate performs domain-level validation on the event payload.
func (e *Event) Validate(maxPayloadBytes int) error {
	if e.TenantID == "" {
		return ErrTenantRequired
	}
	if e.EventType == "" {
		return ErrInvalidEventPayload
	}
	if e.DedupKey == "" {
		return ErrInvalidEventPayload
	}
	if len(e.Payload) > maxPayloadBytes {
		return ErrInvalidEventPayload
	}
	return nil
}
