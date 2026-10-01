package domain

import (
	"fmt"
	"strings"
	"time"
)

// Subscription maps an event_type to a target URL for a specific tenant.
// Subscriptions are strictly scoped to the owning tenant (RULE-SEC-01).
type Subscription struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`

	// Audit columns (RULE-DATA-03)
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// NewSubscription creates a validated Subscription entity.
// Returns ErrInvalidInput if target_url is not an http(s) URL.
func NewSubscription(tenantID, eventType, targetURL, createdBy string) (*Subscription, error) {
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		return nil, fmt.Errorf("%w: target_url must start with http:// or https://", ErrInvalidInput)
	}
	if eventType == "" {
		return nil, fmt.Errorf("%w: event_type must not be empty", ErrInvalidInput)
	}
	return &Subscription{
		TenantID:  tenantID,
		EventType: eventType,
		TargetURL: targetURL,
		CreatedBy: createdBy,
	}, nil
}
