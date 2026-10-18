package domain

import "time"

// OutboxEntry represents a pending delivery entry in the transactional outbox.
type OutboxEntry struct {
	ID               string    `json:"id"`
	EventID          string    `json:"event_id"`
	SubscriptionID   string    `json:"subscription_id"`
	Status           string    `json:"status"` // pending, delivered, failed, dead_letter
	Attempts         int       `json:"attempts"`
	NextAttemptAt    time.Time `json:"next_attempt_at"`
	LastAttemptAt    time.Time `json:"last_attempt_at,omitempty"`
	LastAttemptError string    `json:"last_attempt_error,omitempty"`
	DeliveryID       string    `json:"delivery_id"`
	CreatedAt        time.Time `json:"created_at"`
}
