package domain

import (
	"testing"
	"time"
)

func TestSubscription_NewSubscription(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)
	sub := NewSubscription("tenant-a", "order.created", "https://example.com/webhook", "system", now)

	if sub.TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %s", sub.TenantID)
	}
	if sub.EventType != "order.created" {
		t.Fatalf("expected order.created, got %s", sub.EventType)
	}
	if sub.TargetURL != "https://example.com/webhook" {
		t.Fatalf("unexpected target_url: %s", sub.TargetURL)
	}
	if sub.CreatedBy != "system" {
		t.Fatalf("audit: created_by expected system, got %s", sub.CreatedBy)
	}
	if sub.CreatedAt != now {
		t.Fatalf("audit: created_at mismatch")
	}
	if sub.ID != "" {
		t.Fatalf("new subscription should have empty ID")
	}
}

func TestSubscription_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sub     Subscription
		wantErr error
	}{
		{
			name:    "valid",
			sub:     NewSubscription("t1", "e1", "https://example.com/hook", "s", time.Now()),
			wantErr: nil,
		},
		{
			name:    "missing tenant",
			sub:     NewSubscription("", "e1", "https://example.com/hook", "s", time.Now()),
			wantErr: ErrInvalidInput,
		},
		{
			name:    "missing event_type",
			sub:     NewSubscription("t1", "", "https://example.com/hook", "s", time.Now()),
			wantErr: ErrInvalidInput,
		},
		{
			name:    "missing target_url",
			sub:     NewSubscription("t1", "e1", "", "s", time.Now()),
			wantErr: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.sub.Validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
