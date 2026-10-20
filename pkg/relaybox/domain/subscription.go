package domain

import (
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Subscription represents a webhook subscription that maps an event_type to a
// target URL for a specific tenant (FR-SUB-01). Subscriptions are scoped
// strictly to the owning tenant (FR-SUB-02, RULE-SEC-01).
type Subscription struct {
	// ID is the unique identifier for this subscription (UUIDv7).
	ID string `json:"id" db:"id"`
	// TenantID is the tenant that owns this subscription.
	TenantID string `json:"tenant_id" db:"tenant_id"`
	// EventType is the domain event type this subscription matches.
	EventType string `json:"event_type" db:"event_type"`
	// TargetURL is the HTTP endpoint that will receive the webhook delivery.
	TargetURL string `json:"target_url" db:"target_url"`
	// CreatedAt is the UTC timestamp when the subscription was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// CreatedBy identifies the actor who created this subscription (RULE-DATA-03).
	CreatedBy string `json:"created_by" db:"created_by"`
}

// NewSubscription creates a new Subscription with the given parameters. It
// validates that all required fields are present and that TargetURL is a valid
// absolute URL. Returns ErrInvalidEventPayload on validation failure.
func NewSubscription(tenantID, eventType, targetURL, createdBy string, createdAt time.Time) (*Subscription, error) {
	if tenantID == "" {
		return nil, ErrInvalidEventPayload
	}
	if eventType == "" {
		return nil, ErrInvalidEventPayload
	}
	if targetURL == "" {
		return nil, ErrInvalidEventPayload
	}
	if createdBy == "" {
		return nil, ErrInvalidEventPayload
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil || !parsedURL.IsAbs() {
		return nil, ErrInvalidEventPayload
	}

	return &Subscription{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		EventType: eventType,
		TargetURL: targetURL,
		CreatedAt: createdAt,
		CreatedBy: createdBy,
	}, nil
}
