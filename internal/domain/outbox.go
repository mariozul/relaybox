package domain

import "time"

// Outbox status constants.
const (
	OutboxStatusPending    = "pending"
	OutboxStatusDelivered  = "delivered"
	OutboxStatusFailed     = "failed"
	OutboxStatusDeadLetter = "dead_letter"
)

// Outbox tracks delivery of an event to a subscriber.
type Outbox struct {
	ID             string
	EventID        string
	SubscriptionID string
	Status         string
	Attempts       int
	NextAttemptAt  time.Time
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
