package domain_test

import (
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

func TestSubscriptionEntity(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)

	t.Run("create subscription with all fields", func(t *testing.T) {
		sub := domain.Subscription{
			ID:        "sub-001",
			TenantID:  "tenant-alpha",
			EventType: "order.created",
			TargetURL: "https://example.com/webhook",
			CreatedAt: now,
			CreatedBy: "admin@example.com",
			UpdatedAt: now,
			UpdatedBy: "admin@example.com",
		}

		if sub.ID != "sub-001" {
			t.Errorf("expected ID sub-001, got %s", sub.ID)
		}
		if sub.TenantID != "tenant-alpha" {
			t.Errorf("expected TenantID tenant-alpha, got %s", sub.TenantID)
		}
		if sub.EventType != "order.created" {
			t.Errorf("expected EventType order.created, got %s", sub.EventType)
		}
		if sub.TargetURL != "https://example.com/webhook" {
			t.Errorf("expected TargetURL https://example.com/webhook, got %s", sub.TargetURL)
		}
		if !sub.CreatedAt.Equal(now) {
			t.Errorf("expected CreatedAt %v, got %v", now, sub.CreatedAt)
		}
		if !sub.UpdatedAt.Equal(now) {
			t.Errorf("expected UpdatedAt %v, got %v", now, sub.UpdatedAt)
		}
		if sub.CreatedBy != "admin@example.com" {
			t.Errorf("expected CreatedBy admin@example.com, got %s", sub.CreatedBy)
		}
		if sub.UpdatedBy != "admin@example.com" {
			t.Errorf("expected UpdatedBy admin@example.com, got %s", sub.UpdatedBy)
		}
	})

	t.Run("subscription with empty optional fields", func(t *testing.T) {
		sub := domain.Subscription{
			ID:        "sub-002",
			TenantID:  "tenant-beta",
			EventType: "user.updated",
			TargetURL: "https://other.com/hook",
		}

		if sub.CreatedBy != "" {
			t.Errorf("expected empty CreatedBy, got %s", sub.CreatedBy)
		}
		if sub.UpdatedBy != "" {
			t.Errorf("expected empty UpdatedBy, got %s", sub.UpdatedBy)
		}
	})
}
