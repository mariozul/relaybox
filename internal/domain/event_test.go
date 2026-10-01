package domain

import (
	"testing"
)

func TestEventValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		event   Event
		maxSize int
		wantErr error
	}{
		{"valid", Event{TenantID: "t1", EventType: "x", DedupKey: "dk", Payload: []byte("p")}, 256 * 1024, nil},
		{"no tenant", Event{EventType: "x", DedupKey: "dk", Payload: []byte("p")}, 256 * 1024, ErrTenantRequired},
		{"no type", Event{TenantID: "t1", DedupKey: "dk", Payload: []byte("p")}, 256 * 1024, ErrInvalidEventPayload},
		{"no dedup", Event{TenantID: "t1", EventType: "x", Payload: []byte("p")}, 256 * 1024, ErrInvalidEventPayload},
		{"oversize", Event{TenantID: "t1", EventType: "x", DedupKey: "dk", Payload: make([]byte, 10)}, 5, ErrInvalidEventPayload},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.event.Validate(tt.maxSize)
			if err != tt.wantErr {
				t.Errorf("Validate()=%v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSubscriptionValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		sub  Subscription
		want error
	}{
		{"valid", Subscription{TenantID: "t1", EventType: "x", TargetURL: "http://x"}, nil},
		{"no tenant", Subscription{EventType: "x", TargetURL: "http://x"}, ErrTenantRequired},
		{"no type", Subscription{TenantID: "t1", TargetURL: "http://x"}, ErrInvalidArgument},
		{"no url", Subscription{TenantID: "t1", EventType: "x"}, ErrInvalidArgument},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.sub.Validate(); err != tt.want {
				t.Errorf("Validate()=%v, want %v", err, tt.want)
			}
		})
	}
}

func TestOutboxDeliveryIDStability(t *testing.T) {
	t.Parallel()
	o := &OutboxEntry{ID: "ob-123"}
	if o.DeliveryID() != "ob-123" {
		t.Errorf("DeliveryID=%s, want ob-123", o.DeliveryID())
	}
}
