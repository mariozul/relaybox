package domain

import "time"

// Subscription maps an event_type to a target URL for a tenant.
type Subscription struct {
	ID        string
	TenantID  string
	EventType string
	TargetURL string
	// Audit columns (RULE-DATA-03).
	CreatedBy string
	CreatedAt time.Time
	UpdatedBy string
	UpdatedAt time.Time
}

// NewSubscription creates a valid Subscription with audit metadata.
func NewSubscription(tenantID, eventType, targetURL, createdBy string, now time.Time) Subscription {
	return Subscription{
		TenantID:  tenantID,
		EventType: eventType,
		TargetURL: targetURL,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedBy: createdBy,
		UpdatedAt: now,
	}
}

// Validate checks basic business invariants on the subscription.
func (s Subscription) Validate() error {
	if s.TenantID == "" {
		return ErrInvalidInput
	}
	if s.EventType == "" {
		return ErrInvalidInput
	}
	if s.TargetURL == "" {
		return ErrInvalidInput
	}
	return nil
}
