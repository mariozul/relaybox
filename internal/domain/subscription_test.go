package domain_test

import (
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestSubscription_Fields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := domain.Subscription{
		ID:        "sub_001",
		TenantID:  "tenant_a",
		EventType: "order.created",
		TargetURL: "https://customer.example.com/webhooks",
		CreatedAt: now,
		CreatedBy: "api",
		UpdatedAt: &now,
		UpdatedBy: "",
		DeletedAt: nil,
		DeletedBy: "",
	}

	if sub.ID != "sub_001" {
		t.Errorf("expected ID sub_001, got %s", sub.ID)
	}
	if sub.TenantID != "tenant_a" {
		t.Errorf("expected TenantID tenant_a, got %s", sub.TenantID)
	}
	if sub.EventType != "order.created" {
		t.Errorf("expected EventType order.created, got %s", sub.EventType)
	}
	if sub.TargetURL != "https://customer.example.com/webhooks" {
		t.Errorf("expected TargetURL, got %s", sub.TargetURL)
	}
	if !sub.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, sub.CreatedAt)
	}
	// Audit columns (RULE-DATA-03): all mutation tracking fields present.
	if sub.CreatedBy != "api" {
		t.Errorf("expected CreatedBy api, got %s", sub.CreatedBy)
	}
}
