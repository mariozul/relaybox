package domain

import "time"

// OutboxStatus represents the delivery status of an outbox message.
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusDelivered  OutboxStatus = "delivered"
	OutboxStatusDeadLetter OutboxStatus = "deadletter"
)

// OutboxMessage represents a durable delivery record for an event to a subscription.
type OutboxMessage struct {
	ID             string
	EventID        string
	SubscriptionID string
	Status         OutboxStatus
	Attempts       int
	NextAttemptAt  time.Time
	DeliveryID     string
	// Audit columns (RULE-DATA-03).
	CreatedBy string
	CreatedAt time.Time
	UpdatedBy string
	UpdatedAt time.Time
}

// NewOutboxMessage creates a pending OutboxMessage with audit metadata.
func NewOutboxMessage(eventID, subscriptionID, deliveryID, createdBy string, now time.Time) OutboxMessage {
	return OutboxMessage{
		EventID:        eventID,
		SubscriptionID: subscriptionID,
		Status:         OutboxStatusPending,
		Attempts:       0,
		NextAttemptAt:  now,
		DeliveryID:     deliveryID,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedBy:      createdBy,
		UpdatedAt:      now,
	}
}

// IsPending returns true when the message has not yet been delivered or dead-lettered.
func (o OutboxMessage) IsPending() bool { return o.Status == OutboxStatusPending }
