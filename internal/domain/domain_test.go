package domain

import (
	"encoding/json"
	"testing"
)

func TestNewEventRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, eventType, dedup string
		payload                json.RawMessage
	}{
		{"empty type", "", "key", json.RawMessage(`{}`)},
		{"empty key", "created", "", json.RawMessage(`{}`)},
		{"invalid payload", "created", "key", json.RawMessage(`{`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewEvent(tc.eventType, tc.payload, tc.dedup); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNewSubscriptionValidatesURL(t *testing.T) {
	t.Parallel()
	if _, err := NewSubscription("created", "file:///secret"); err == nil {
		t.Fatal("expected URL validation error")
	}
	if _, err := NewSubscription("created", "https://example.com/hook"); err != nil {
		t.Fatalf("expected valid subscription: %v", err)
	}
}
