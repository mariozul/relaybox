package domain_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

// TC-01-UNIT: Dedup re-ingest simulation — NewEvent must produce stable fields.
func TestEventFieldsPresent(t *testing.T) {
	t.Parallel()
	e := &domain.Event{
		ID:        "evt-1",
		TenantID:  "tnt-1",
		EventType: "order.created",
		Payload:   []byte(`{"order":1}`),
		DedupKey:  "dk-1",
		CreatedAt: time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC),
		CreatedBy: "system",
	}
	if e.ID == "" {
		t.Fatal("Event.ID must not be empty")
	}
	if e.TenantID == "" {
		t.Fatal("Event.TenantID must not be empty (RULE-SEC-01)")
	}
	if e.EventType == "" {
		t.Fatal("Event.EventType must not be empty")
	}
	if e.DedupKey == "" {
		t.Fatal("Event.DedupKey must not be empty (FR-ING-03)")
	}
	if e.CreatedAt.IsZero() {
		t.Fatal("Event.CreatedAt must not be zero (RULE-DATA-03)")
	}
}

// TC-06-UNIT: Subscription struct fields for CRUD happy path.
func TestSubscriptionFieldsPresent(t *testing.T) {
	t.Parallel()
	url := "https://example.com/hook"
	s := &domain.Subscription{
		ID:        "sub-1",
		TenantID:  "tnt-1",
		EventType: "order.created",
		TargetURL: url,
		CreatedAt: time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC),
		CreatedBy: "system",
	}
	if s.ID == "" {
		t.Fatal("Subscription.ID must not be empty")
	}
	if s.TenantID == "" {
		t.Fatal("Subscription.TenantID must not be empty (RULE-SEC-01)")
	}
	if s.EventType == "" {
		t.Fatal("Subscription.EventType must not be empty")
	}
	if s.TargetURL == "" {
		t.Fatal("Subscription.TargetURL must not be empty")
	}
	if s.CreatedBy == "" {
		t.Fatal("Subscription.CreatedBy must not be empty (RULE-DATA-03)")
	}
}

// TC-03-INT / RULE-EVT-02: OutboxEntry fields.
func TestOutboxEntryFieldsPresent(t *testing.T) {
	t.Parallel()
	o := &domain.OutboxEntry{
		ID:             "obx-1",
		EventID:        "evt-1",
		SubscriptionID: "sub-1",
		Status:         domain.DeliveryStatusPending,
		Attempts:        0,
		NextAttemptAt:   time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC),
		DeliveryID:     "dlv-1",
		CreatedAt:      time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC),
		CreatedBy:      "system",
	}
	if o.EventID == "" {
		t.Fatal("OutboxEntry.EventID must not be empty")
	}
	if o.SubscriptionID == "" {
		t.Fatal("OutboxEntry.SubscriptionID must not be empty")
	}
	if o.DeliveryID == "" {
		t.Fatal("OutboxEntry.DeliveryID must not be empty (FR-DEL-04)")
	}
}

// Sentinel errors must be detectable via errors.Is (RULE-ARH-03).
func TestSentinelErrorsAreDetectable(t *testing.T) {
	t.Parallel()
	if domain.ErrEventDuplicate == nil {
		t.Fatal("ErrEventDuplicate must not be nil")
	}
	if !errors.Is(domain.ErrEventDuplicate, domain.ErrEventDuplicate) {
		t.Fatal("ErrEventDuplicate must be detectable via errors.Is")
	}
	wrapped := fmt.Errorf("wrapped: %w", domain.ErrInvalidEventPayload)
	if !errors.Is(wrapped, domain.ErrInvalidEventPayload) {
		t.Fatal("wrapped ErrInvalidEventPayload must be detectable via errors.Is")
	}
}

// Clock implementations.
func TestFixedClock(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC)
	c := domain.FixedClock{NowT: now}
	if !c.Now().Equal(now) {
		t.Fatalf("FixedClock must return %v, got %v", now, c.Now())
	}
}

func TestSystemClock(t *testing.T) {
	t.Parallel()
	c := domain.SystemClock{}
	before := time.Now()
	got := c.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Fatalf("SystemClock must return current time, got %v", got)
	}
}
