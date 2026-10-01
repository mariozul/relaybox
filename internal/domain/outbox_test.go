package domain_test

import (
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestOutbox_Fields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	nextAttempt := now.Add(5 * time.Second)
	ob := domain.Outbox{
		ID:             "ob_001",
		EventID:        "evt_001",
		SubscriptionID: "sub_001",
		Status:         domain.OutboxStatusPending,
		Attempts:       0,
		NextAttemptAt:  nextAttempt,
		LastError:      "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if ob.ID != "ob_001" {
		t.Errorf("expected ID ob_001, got %s", ob.ID)
	}
	if ob.Status != domain.OutboxStatusPending {
		t.Errorf("expected status pending, got %s", ob.Status)
	}
	if ob.Attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", ob.Attempts)
	}
	if !ob.NextAttemptAt.Equal(nextAttempt) {
		t.Errorf("expected NextAttemptAt %v, got %v", nextAttempt, ob.NextAttemptAt)
	}
}

func TestOutboxStatusConstants(t *testing.T) {
	t.Parallel()

	if domain.OutboxStatusPending != "pending" {
		t.Errorf("expected pending, got %s", domain.OutboxStatusPending)
	}
	if domain.OutboxStatusDelivered != "delivered" {
		t.Errorf("expected delivered, got %s", domain.OutboxStatusDelivered)
	}
	if domain.OutboxStatusFailed != "failed" {
		t.Errorf("expected failed, got %s", domain.OutboxStatusFailed)
	}
	if domain.OutboxStatusDeadLetter != "dead_letter" {
		t.Errorf("expected dead_letter, got %s", domain.OutboxStatusDeadLetter)
	}
}
