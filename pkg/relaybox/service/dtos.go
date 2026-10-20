// Package service implements the core business logic for the Relaybox webhook
// relay service. It orchestrates domain operations by coordinating repository
// interfaces and enforcing business invariants (Clean Architecture service layer).
package service

import (
	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// IngestEventRequest represents the input for ingesting a new event into the
// Relaybox system (FR-ING-01). It captures the event metadata required to
// persist and route the event to matching subscribers.
type IngestEventRequest struct {
	// EventType is the domain event type (e.g., "order.created").
	EventType string `json:"event_type"`
	// Payload is the raw JSON payload of the event.
	Payload []byte `json:"payload"`
	// DedupKey is the deduplication key for idempotent ingestion
	// (FR-ING-03, RULE-DATA-02).
	DedupKey string `json:"dedup_key"`
}

// IngestEventResponse represents the result of a successful event ingestion.
type IngestEventResponse struct {
	// EventID is the unique identifier assigned to the ingested event.
	EventID string `json:"event_id"`
}

// SubscriptionResponse represents a subscription in API responses.
type SubscriptionResponse struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
}

// SubscriptionToResponse converts a domain Subscription to a response DTO.
func SubscriptionToResponse(sub *domain.Subscription) SubscriptionResponse {
	return SubscriptionResponse{
		ID:        sub.ID,
		TenantID:  sub.TenantID,
		EventType: sub.EventType,
		TargetURL: sub.TargetURL,
		CreatedAt: sub.CreatedAt.Format(timeLayout),
		CreatedBy: sub.CreatedBy,
	}
}

// timeLayout is the ISO 8601 UTC timestamp format used in API responses.
const timeLayout = "2006-01-02T15:04:05.000Z"
