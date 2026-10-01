package domain

import (
	"testing"
	"time"
)

func TestOutboxMessage_NewOutboxMessage(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)
	msg := NewOutboxMessage("evt-1", "sub-1", "del-1", "system", now)

	if msg.EventID != "evt-1" {
		t.Fatalf("expected evt-1, got %s", msg.EventID)
	}
	if msg.SubscriptionID != "sub-1" {
		t.Fatalf("expected sub-1, got %s", msg.SubscriptionID)
	}
	if msg.Status != OutboxStatusPending {
		t.Fatalf("expected pending, got %s", msg.Status)
	}
	if msg.Attempts != 0 {
		t.Fatalf("expected 0 attempts, got %d", msg.Attempts)
	}
	if msg.NextAttemptAt != now {
		t.Fatalf("expected NextAttemptAt to be now")
	}
	if msg.DeliveryID != "del-1" {
		t.Fatalf("expected del-1, got %s", msg.DeliveryID)
	}
	if msg.CreatedBy != "system" {
		t.Fatalf("audit: created_by expected system, got %s", msg.CreatedBy)
	}
	if msg.CreatedAt != now {
		t.Fatalf("audit: created_at mismatch")
	}
	if !msg.IsPending() {
		t.Fatal("new outbox message should be pending")
	}
}

func TestOutboxMessage_IsPending(t *testing.T) {
	t.Parallel()

	msg := NewOutboxMessage("e1", "s1", "d1", "sys", time.Now())
	if !msg.IsPending() {
		t.Fatal("fresh message must be pending")
	}

	msg.Status = OutboxStatusDelivered
	if msg.IsPending() {
		t.Fatal("delivered message must not be pending")
	}

	msg.Status = OutboxStatusDeadLetter
	if msg.IsPending() {
		t.Fatal("dead-letter message must not be pending")
	}
}
