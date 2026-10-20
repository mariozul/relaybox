package domain

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OutboxStatus represents the lifecycle status of an outbox entry.
type OutboxStatus string

const (
	// OutboxStatusPending indicates the entry is queued for delivery.
	OutboxStatusPending OutboxStatus = "pending"
	// OutboxStatusDelivered indicates the event was successfully delivered.
	OutboxStatusDelivered OutboxStatus = "delivered"
	// OutboxStatusFailed indicates the delivery attempt failed and will be retried.
	OutboxStatusFailed OutboxStatus = "failed"
	// OutboxStatusDeadLetter indicates the entry exhausted retries and was dead-lettered.
	OutboxStatusDeadLetter OutboxStatus = "deadletter"

	// MaxDeliveryAttempts is the maximum number of delivery attempts before
	// the entry is dead-lettered.
	MaxDeliveryAttempts = 5
)

// OutboxEntry represents a pending delivery of an event to a subscription
// (FR-ING-02, RULE-EVT-02). It tracks the delivery lifecycle including
// retry attempts and status transitions.
type OutboxEntry struct {
	// ID is the unique identifier for this outbox entry (UUIDv7).
	ID string `json:"id" db:"id"`
	// EventID is the ID of the event to deliver.
	EventID string `json:"event_id" db:"event_id"`
	// SubscriptionID is the ID of the target subscription.
	SubscriptionID string `json:"subscription_id" db:"subscription_id"`
	// Status is the current delivery status (pending, delivered, failed, deadletter).
	Status OutboxStatus `json:"status" db:"status"`
	// Attempts is the number of delivery attempts made so far.
	Attempts int `json:"attempts" db:"attempts"`
	// NextAttemptAt is the UTC timestamp for the next scheduled retry, if applicable.
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty" db:"next_attempt_at"`
	// CreatedAt is the UTC timestamp when the outbox entry was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewOutboxEntry creates a new OutboxEntry with status Pending and zero attempts.
// It validates that EventID and SubscriptionID are non-empty.
func NewOutboxEntry(eventID, subscriptionID string, createdAt time.Time) (*OutboxEntry, error) {
	if eventID == "" {
		return nil, ErrInvalidEventPayload
	}
	if subscriptionID == "" {
		return nil, ErrInvalidEventPayload
	}

	return &OutboxEntry{
		ID:             uuid.NewString(),
		EventID:        eventID,
		SubscriptionID: subscriptionID,
		Status:         OutboxStatusPending,
		Attempts:       0,
		NextAttemptAt:  nil,
		CreatedAt:      createdAt,
	}, nil
}

// AdvanceAttempt transitions the outbox entry's status after a delivery
// attempt. If the new status is Delivered, the entry is marked as delivered.
// Otherwise, attempts are incremented. If attempts reach maxAttempts, the
// entry is dead-lettered. If the entry is already delivered, no changes are
// made.
func (e *OutboxEntry) AdvanceAttempt(newStatus OutboxStatus, maxAttempts int) error {
	if e.Status == OutboxStatusDelivered {
		return nil
	}

	if newStatus == OutboxStatusDelivered {
		e.Status = OutboxStatusDelivered
		return nil
	}

	e.Attempts++
	if e.Attempts >= maxAttempts {
		e.Status = OutboxStatusDeadLetter
		return nil
	}

	e.Status = newStatus
	return nil
}

// GenerateDeliveryID creates a deterministic delivery ID from an event ID and
// subscription ID using SHA-256 hashing (FR-DEL-04, RULE-DATA-02). The same
// inputs always produce the same output, ensuring the X-Relaybox-Delivery-Id
// header is stable across retries.
func GenerateDeliveryID(eventID, subscriptionID string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", eventID, subscriptionID)))
	return fmt.Sprintf("dlv-%x", h[:16])
}
