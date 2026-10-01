package service

import (
	"context"
	"fmt"

	"github.com/mariozul/relaybox/internal/domain"
)

// EventService handles event ingestion with idempotency and transactional outbox.
type EventService struct {
	events       domain.EventStore
	outbox       domain.OutboxStore
	subscriptions domain.SubscriptionStore
	tx           domain.TxManager
}

// NewEventService creates a new EventService.
func NewEventService(events domain.EventStore, outbox domain.OutboxStore, subs domain.SubscriptionStore, tx domain.TxManager) *EventService {
	return &EventService{events: events, outbox: outbox, subscriptions: subs, tx: tx}
}

// Ingest persists an event and creates outbox entries in a single transaction (FR-ING-02, RULE-DATA-01).
// Idempotent per (tenant_id, dedup_key) — duplicate calls return the existing event (FR-ING-03, RULE-DATA-02).
func (s *EventService) Ingest(ctx context.Context, tenantID, eventType string, payload []byte, dedupKey string) (domain.Event, error) {
	// Check idempotency first (outside tx is OK — unique constraint is the real guard).
	existing, err := s.events.GetEventByDedup(ctx, tenantID, dedupKey)
	if err == nil {
		return existing, nil
	}

	var event domain.Event
	err = s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		// Persist the event (idempotent via unique constraint).
		evt, createErr := s.events.CreateEvent(txCtx, tenantID, eventType, payload, dedupKey)
		if createErr != nil {
			return fmt.Errorf("create event: %w", createErr)
		}
		event = evt

		// Find matching subscriptions.
		subs, subsErr := s.subscriptions.GetSubscriptionsByEventType(txCtx, tenantID, eventType)
		if subsErr != nil {
			return fmt.Errorf("get subscriptions: %w", subsErr)
		}

		// Create outbox entries for each matching subscription.
		for _, sub := range subs {
			if _, obErr := s.outbox.CreateOutbox(txCtx, event.ID, sub.ID); obErr != nil {
				return fmt.Errorf("create outbox for sub %s: %w", sub.ID, obErr)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Event{}, err
	}
	return event, nil
}
