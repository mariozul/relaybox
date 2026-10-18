package domain_test

import (
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

func TestEventEntity(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)

	t.Run("create event with all fields", func(t *testing.T) {
		event := domain.Event{
			ID:        "evt-001",
			TenantID:  "tenant-alpha",
			EventType: "order.created",
			Payload:   []byte(`{"order_id":"ord-123"}`),
			DedupKey:  "dedup-abc-123",
			CreatedAt: now,
			CreatedBy: "system",
		}

		if event.ID != "evt-001" {
			t.Errorf("expected ID evt-001, got %s", event.ID)
		}
		if event.TenantID != "tenant-alpha" {
			t.Errorf("expected TenantID tenant-alpha, got %s", event.TenantID)
		}
		if event.EventType != "order.created" {
			t.Errorf("expected EventType order.created, got %s", event.EventType)
		}
		if string(event.Payload) != `{"order_id":"ord-123"}` {
			t.Errorf("expected Payload {\"order_id\":\"ord-123\"}, got %s", string(event.Payload))
		}
		if event.DedupKey != "dedup-abc-123" {
			t.Errorf("expected DedupKey dedup-abc-123, got %s", event.DedupKey)
		}
		if !event.CreatedAt.Equal(now) {
			t.Errorf("expected CreatedAt %v, got %v", now, event.CreatedAt)
		}
		if event.CreatedBy != "system" {
			t.Errorf("expected CreatedBy system, got %s", event.CreatedBy)
		}
	})

	t.Run("event with empty optional fields", func(t *testing.T) {
		event := domain.Event{
			ID:        "evt-002",
			TenantID:  "tenant-beta",
			EventType: "user.updated",
			Payload:   []byte(`{}`),
		}

		if event.DedupKey != "" {
			t.Errorf("expected empty DedupKey, got %s", event.DedupKey)
		}
		if event.CreatedBy != "" {
			t.Errorf("expected empty CreatedBy, got %s", event.CreatedBy)
		}
	})
}
