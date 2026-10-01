package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNewEventValid(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{"order_id": "123"}`)
	e, err := NewEvent("t1", "order.created", payload, "dk1", "sys")
	if err != nil {
		t.Fatalf("NewEvent returned error: %v", err)
	}
	if e.TenantID != "t1" || e.EventType != "order.created" || e.DedupKey != "dk1" || e.CreatedBy != "sys" {
		t.Error("Event fields not set correctly")
	}
	if string(e.Payload) != `{"order_id": "123"}` {
		t.Errorf("Payload mismatch: %s", e.Payload)
	}
}

func TestNewEventMissingEventType(t *testing.T) {
	t.Parallel()

	_, err := NewEvent("t1", "", json.RawMessage(`{}`), "dk", "sys")
	if err == nil {
		t.Fatal("expected error for empty event_type")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewEventMissingPayload(t *testing.T) {
	t.Parallel()

	_, err := NewEvent("t1", "order.created", nil, "dk", "sys")
	if err == nil {
		t.Fatal("expected error for nil payload")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewEventOptionalDedupKey(t *testing.T) {
	t.Parallel()

	// dedup_key is optional; empty string is valid
	e, err := NewEvent("t1", "order.created", json.RawMessage(`{}`), "", "sys")
	if err != nil {
		t.Fatalf("NewEvent with empty dedup_key should not error: %v", err)
	}
	if e.DedupKey != "" {
		t.Errorf("expected empty dedup_key, got %q", e.DedupKey)
	}
}

func TestNewSubscriptionValid(t *testing.T) {
	t.Parallel()

	s, err := NewSubscription("t1", "order.created", "https://example.com/webhook", "sys")
	if err != nil {
		t.Fatalf("NewSubscription returned error: %v", err)
	}
	if s.TenantID != "t1" || s.EventType != "order.created" || s.TargetURL != "https://example.com/webhook" {
		t.Error("Subscription fields not set correctly")
	}
}

func TestNewSubscriptionInvalidURL(t *testing.T) {
	t.Parallel()

	invalidURLs := []string{"ftp://example.com/webhook", "ws://example.com", "", "example.com/no-scheme"}
	for _, url := range invalidURLs {
		_, err := NewSubscription("t1", "order.created", url, "sys")
		if err == nil {
			t.Errorf("expected error for URL %q", url)
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for URL %q, got %v", url, err)
		}
	}
}

func TestNewSubscriptionMissingEventType(t *testing.T) {
	t.Parallel()

	_, err := NewSubscription("t1", "", "https://example.com/webhook", "sys")
	if err == nil {
		t.Fatal("expected error for empty event_type")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestOutboxStatusConstants(t *testing.T) {
	t.Parallel()

	seen := map[OutboxStatus]bool{}
	for _, s := range []OutboxStatus{OutboxStatusPending, OutboxStatusDelivered, OutboxStatusFailed, OutboxStatusDeadLetter} {
		if seen[s] {
			t.Errorf("duplicate status value: %s", s)
		}
		seen[s] = true
	}
}

func TestDeliveryIDFromEntryID(t *testing.T) {
	t.Parallel()

	// Same entry ID → same DeliveryID (stable across retries — TC-11)
	id1 := DeliveryIDFromEntryID("entry-001")
	id2 := DeliveryIDFromEntryID("entry-001")
	if id1 != id2 {
		t.Fatalf("DeliveryID must be deterministic: %s != %s", id1, id2)
	}

	// Different entry IDs → different DeliveryIDs
	id3 := DeliveryIDFromEntryID("entry-002")
	if id1 == id3 {
		t.Fatal("different entry IDs must produce different DeliveryIDs")
	}

	// Non-empty
	if id1 == "" || id3 == "" {
		t.Error("DeliveryID must not be empty")
	}
}
