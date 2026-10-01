package domain

import "time"

// OutboxStatus represents the delivery state of an outbox row.
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusDelivered  OutboxStatus = "delivered"
	OutboxStatusFailed     OutboxStatus = "failed"
	OutboxStatusDeadLetter OutboxStatus = "dead_letter"
)

// OutboxEntry is a row in the transactional outbox table, representing
// a single delivery attempt for an event to a subscription endpoint.
type OutboxEntry struct {
	ID             string
	EventID        string
	SubscriptionID string
	Status         OutboxStatus
	Attempts       int
	NextAttemptAt  time.Time
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DeliveryID generates a stable delivery identifier from the outbox entry ID
// for the X-Relaybox-Delivery-Id header (FR-DEL-04).
func (o *OutboxEntry) DeliveryID() string {
	return o.ID
}
