package service

import (
	"context"
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestEventService_Ingest_Success(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	subRepo := newMockSubscriptionRepo()

	// Pre-register a subscription matching the event type.
	_, err := subRepo.Create(context.Background(), domain.NewSubscription("tenant-a", "order.created", "https://example.com/hook", "system", clock.Now()))
	if err != nil {
		t.Fatalf("pre-create sub: %v", err)
	}

	svc := NewEventService(eventRepo, outboxRepo, subRepo, clock)

	result, err := svc.Ingest(context.Background(), IngestInput{
		TenantID:  "tenant-a",
		EventType: "order.created",
		Payload:   []byte(`{"order":1}`),
		DedupKey:  "dk-1",
		CreatedBy: "system",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventID == "" {
		t.Fatal("expected non-empty event ID")
	}
	if result.EventID == "" {
		t.Fatal("expected non-empty event ID")
	}
	if result.OutboxRowCount != 1 {
		t.Fatalf("expected 1 outbox row, got %d", result.OutboxRowCount)
	}
	if result.OutboxRowCount != 1 {
		t.Fatalf("expected 1 outbox row, got %d", result.OutboxRowCount)
	}
}

func TestEventService_Ingest_Idempotent(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	subRepo := newMockSubscriptionRepo()

	_, err := subRepo.Create(context.Background(), domain.NewSubscription("tenant-a", "order.created", "https://example.com/hook", "system", clock.Now()))
	if err != nil {
		t.Fatalf("pre-create sub: %v", err)
	}

	svc := NewEventService(eventRepo, outboxRepo, subRepo, clock)
	input := IngestInput{
		TenantID:  "tenant-a",
		EventType: "order.created",
		Payload:   []byte(`{"order":1}`),
		DedupKey:  "dk-1",
		CreatedBy: "system",
	}

	// First ingest.
	r1, err := svc.Ingest(context.Background(), input)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	// Second ingest with same dedup key.
	r2, err := svc.Ingest(context.Background(), input)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	// Both should return the same event ID.
	if r1.EventID != r2.EventID {
		t.Fatalf("idempotent ingest returned different IDs: %s vs %s", r1.EventID, r2.EventID)
	}
}

func TestEventService_Ingest_ValidationError(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	svc := NewEventService(newMockEventRepo(), newMockOutboxRepo(), newMockSubscriptionRepo(), clock)

	_, err := svc.Ingest(context.Background(), IngestInput{
		TenantID:  "",
		EventType: "e1",
		Payload:   []byte("p"),
		DedupKey:  "dk",
		CreatedBy: "s",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !domain.IsInvalidInput(err) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestEventService_Ingest_NoMatchingSubscriptions(t *testing.T) {
	t.Parallel()

	clock := newMockClock()
	eventRepo := newMockEventRepo()
	outboxRepo := newMockOutboxRepo()
	subRepo := newMockSubscriptionRepo()

	svc := NewEventService(eventRepo, outboxRepo, subRepo, clock)

	result, err := svc.Ingest(context.Background(), IngestInput{
		TenantID:  "tenant-a",
		EventType: "order.created",
		Payload:   []byte(`{"order":1}`),
		DedupKey:  "dk-1",
		CreatedBy: "system",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OutboxRowCount != 0 {
		t.Fatalf("expected 0 outbox rows with no matching subs, got %d", result.OutboxRowCount)
	}
}
