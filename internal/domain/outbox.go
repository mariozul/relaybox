package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// OutboxStatus represents the delivery status of an outbox row.
type OutboxStatus string

const (
	// OutboxStatusPending indicates the row is waiting to be delivered.
	OutboxStatusPending OutboxStatus = "pending"

	// OutboxStatusDelivered indicates successful delivery to the subscriber.
	OutboxStatusDelivered OutboxStatus = "delivered"

	// OutboxStatusFailed indicates a transient delivery failure; retry pending.
	OutboxStatusFailed OutboxStatus = "failed"

	// OutboxStatusDeadLetter indicates the row has been permanently failed
	// after exceeding retry attempts or receiving a non-retryable error.
	OutboxStatusDeadLetter OutboxStatus = "deadletter"
)

// OutboxEntry represents a single delivery attempt entry in the outbox.
// It tracks the relationship between an event, a subscription, and the delivery state.
type OutboxEntry struct {
	ID             string       `json:"id"`
	EventID        string       `json:"event_id"`
	SubscriptionID string       `json:"subscription_id"`
	Status         OutboxStatus `json:"status"`
	Attempts       int          `json:"attempts"`
	NextAttemptAt  time.Time    `json:"next_attempt_at"`
	DeliveryID     string       `json:"delivery_id"`

	// Audit columns (RULE-DATA-03)
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// DeliveryIDFromEntryID derives a stable, deterministic DeliveryID from the
// outbox entry ID. This ensures the same DeliveryID across all retry attempts
// for a given outbox row (TC-11, RULE-DATA-02).
func DeliveryIDFromEntryID(entryID string) string {
	h := sha256.Sum256([]byte(entryID))
	return hex.EncodeToString(h[:])
}

// MarkDelivered transitions the outbox entry to delivered status.
// Idempotent per RULE-DATA-02: no-op if already delivered.
func (o *OutboxEntry) MarkDelivered() error {
	if o.Status == OutboxStatusDelivered {
		return nil // idempotent
	}
	if o.Status == OutboxStatusDeadLetter {
		return fmt.Errorf("%w: cannot deliver a dead-lettered entry", ErrInvalidState)
	}
	o.Status = OutboxStatusDelivered
	return nil
}

// MarkFailed transitions the outbox entry to failed (transient) status.
func (o *OutboxEntry) MarkFailed() error {
	if o.Status == OutboxStatusDelivered || o.Status == OutboxStatusDeadLetter {
		return fmt.Errorf("%w: cannot mark terminal entry as failed", ErrInvalidState)
	}
	o.Status = OutboxStatusFailed
	return nil
}

// MarkDeadLetter transitions the outbox entry to deadletter status.
// Idempotent per RULE-DATA-02: no-op if already dead-lettered.
func (o *OutboxEntry) MarkDeadLetter() error {
	if o.Status == OutboxStatusDeadLetter {
		return nil // idempotent
	}
	if o.Status == OutboxStatusDelivered {
		return fmt.Errorf("%w: cannot dead-letter a delivered entry", ErrInvalidState)
	}
	o.Status = OutboxStatusDeadLetter
	return nil
}

// IncrementAttempts bumps the attempt counter and recomputes NextAttemptAt
// using the exponential backoff function.
func (o *OutboxEntry) IncrementAttempts(clock Clock) error {
	if o.Status == OutboxStatusDelivered || o.Status == OutboxStatusDeadLetter {
		return fmt.Errorf("%w: cannot increment attempts on a terminal entry", ErrInvalidState)
	}
	o.Attempts++
	o.NextAttemptAt = clock.Now().Add(ExponentialBackoff(1*time.Second, 1*time.Hour, 2.0, o.Attempts))
	return nil
}

// IsTerminal returns true when the delivery has reached a final state.
func (o *OutboxEntry) IsTerminal() bool {
	return o.Status == OutboxStatusDelivered || o.Status == OutboxStatusDeadLetter
}
