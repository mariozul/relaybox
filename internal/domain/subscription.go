package domain

import "time"

// Subscription maps an event_type to a target URL for a given tenant.
type Subscription struct {
	ID        string
	TenantID  string
	EventType string
	TargetURL string
	CreatedAt time.Time
	CreatedBy string
}

// Validate performs domain-level validation on the subscription.
func (s *Subscription) Validate() error {
	if s.TenantID == "" {
		return ErrTenantRequired
	}
	if s.EventType == "" {
		return ErrInvalidArgument
	}
	if s.TargetURL == "" {
		return ErrInvalidArgument
	}
	return nil
}
