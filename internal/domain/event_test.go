package domain_test

import (
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestEvent_Fields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	evt := domain.Event{
		ID:        "evt_001",
		TenantID:  "tenant_a",
		EventType: "order.created",
		Payload:   []byte(`{"order_id":"1"}`),
		DedupKey:  "idem_001",
		CreatedAt: now,
		CreatedBy: "system",
	}

	if evt.ID != "evt_001" {
		t.Errorf("expected ID evt_001, got %s", evt.ID)
	}
	if evt.TenantID != "tenant_a" {
		t.Errorf("expected TenantID tenant_a, got %s", evt.TenantID)
	}
	if evt.EventType != "order.created" {
		t.Errorf("expected EventType order.created, got %s", evt.EventType)
	}
	if string(evt.Payload) != `{"order_id":"1"}` {
		t.Errorf("unexpected payload: %s", string(evt.Payload))
	}
	if evt.DedupKey != "idem_001" {
		t.Errorf("expected DedupKey idem_001, got %s", evt.DedupKey)
	}
	if !evt.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, evt.CreatedAt)
	}
	if evt.CreatedBy != "system" {
		t.Errorf("expected CreatedBy system, got %s", evt.CreatedBy)
	}
}

func TestEvent_AuditColumn(t *testing.T) {
	t.Parallel()
	// RULE-DATA-03: CreatedBy must be present.
	evt := domain.Event{CreatedBy: "test-user"}
	if evt.CreatedBy != "test-user" {
		t.Error("CreatedBy audit column missing")
	}
}
