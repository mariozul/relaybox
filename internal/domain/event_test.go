package domain

import (
	"testing"
	"time"
)

func TestEvent_NewEvent(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 5, 20, 12, 0, 0, 0, time.UTC)
	e := NewEvent("tenant-a", "order.created", []byte(`{"order":1}`), "dk-1", "system", now)

	if e.TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %s", e.TenantID)
	}
	if e.EventType != "order.created" {
		t.Fatalf("expected order.created, got %s", e.EventType)
	}
	if string(e.Payload) != `{"order":1}` {
		t.Fatalf("unexpected payload: %s", string(e.Payload))
	}
	if e.DedupKey != "dk-1" {
		t.Fatalf("expected dk-1, got %s", e.DedupKey)
	}
	if e.CreatedBy != "system" {
		t.Fatalf("audit: created_by expected system, got %s", e.CreatedBy)
	}
	if e.CreatedAt != now {
		t.Fatalf("audit: created_at mismatch")
	}
	if e.UpdatedBy != "system" {
		t.Fatalf("audit: updated_by expected system, got %s", e.UpdatedBy)
	}
	if e.UpdatedAt != now {
		t.Fatalf("audit: updated_at mismatch")
	}
	if e.ID != "" {
		t.Fatalf("new event should have empty ID")
	}
}

func TestEvent_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		event   Event
		wantErr error
	}{
		{
			name:    "valid",
			event:   NewEvent("t1", "e1", []byte("p"), "dk", "s", time.Now()),
			wantErr: nil,
		},
		{
			name:    "missing tenant",
			event:   NewEvent("", "e1", []byte("p"), "dk", "s", time.Now()),
			wantErr: ErrInvalidInput,
		},
		{
			name:    "missing event_type",
			event:   NewEvent("t1", "", []byte("p"), "dk", "s", time.Now()),
			wantErr: ErrInvalidInput,
		},
		{
			name:    "empty payload",
			event:   NewEvent("t1", "e1", nil, "dk", "s", time.Now()),
			wantErr: ErrInvalidInput,
		},
		{
			name:    "missing dedup_key",
			event:   NewEvent("t1", "e1", []byte("p"), "", "s", time.Now()),
			wantErr: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.event.Validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
