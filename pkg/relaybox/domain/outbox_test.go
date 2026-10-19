package domain_test

import (
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

func TestOutboxEntryEntity(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)

	t.Run("create outbox entry with all fields", func(t *testing.T) {
		entry := domain.OutboxEntry{
			ID:               "ob-001",
			EventID:          "evt-001",
			SubscriptionID:   "sub-001",
			Status:           domain.OutboxStatusPending,
			Attempts:         0,
			NextAttemptAt:    now,
			LastAttemptAt:    now,
			LastAttemptError: "",
			DeliveryID:       "del-abc-123",
			CreatedAt:        now,
		}

		if entry.ID != "ob-001" {
			t.Errorf("expected ID ob-001, got %s", entry.ID)
		}
		if entry.EventID != "evt-001" {
			t.Errorf("expected EventID evt-001, got %s", entry.EventID)
		}
		if entry.SubscriptionID != "sub-001" {
			t.Errorf("expected SubscriptionID sub-001, got %s", entry.SubscriptionID)
		}
		if entry.Status != domain.OutboxStatusPending {
			t.Errorf("expected Status %s, got %s", domain.OutboxStatusPending, entry.Status)
		}
		if entry.Attempts != 0 {
			t.Errorf("expected Attempts 0, got %d", entry.Attempts)
		}
		if entry.DeliveryID != "del-abc-123" {
			t.Errorf("expected DeliveryID del-abc-123, got %s", entry.DeliveryID)
		}
	})

	t.Run("outbox entry after delivery attempt", func(t *testing.T) {
		entry := domain.OutboxEntry{
			ID:               "ob-002",
			EventID:          "evt-001",
			SubscriptionID:   "sub-002",
			Status:           domain.OutboxStatusFailed,
			Attempts:         3,
			NextAttemptAt:    now.Add(5 * time.Minute),
			LastAttemptAt:    now,
			LastAttemptError: "connection timeout",
			DeliveryID:       "del-def-456",
			CreatedAt:        now.Add(-10 * time.Minute),
		}

		if entry.Status != domain.OutboxStatusFailed {
			t.Errorf("expected Status %s, got %s", domain.OutboxStatusFailed, entry.Status)
		}
		if entry.Attempts != 3 {
			t.Errorf("expected Attempts 3, got %d", entry.Attempts)
		}
		if entry.LastAttemptError != "connection timeout" {
			t.Errorf("expected LastAttemptError 'connection timeout', got %s", entry.LastAttemptError)
		}
	})
}

func TestOutboxStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		got   string
		want  string
	}{
		{name: "OutboxStatusPending", got: domain.OutboxStatusPending, want: "pending"},
		{name: "OutboxStatusDelivered", got: domain.OutboxStatusDelivered, want: "delivered"},
		{name: "OutboxStatusFailed", got: domain.OutboxStatusFailed, want: "failed"},
		{name: "OutboxStatusDeadLetter", got: domain.OutboxStatusDeadLetter, want: "dead_letter"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("outbox status constant %s = %s, want %s", tc.name, tc.got, tc.want)
			}
		})
	}
}
