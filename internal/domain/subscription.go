package domain

import "time"

// Subscription maps an event_type to a target URL for a tenant.
// Includes full audit trail per RULE-DATA-03 (CreatedBy, UpdatedBy, DeletedBy + timestamps).
type Subscription struct {
	ID        string
	TenantID  string
	EventType string
	TargetURL string
	CreatedAt time.Time
	CreatedBy string
	UpdatedAt *time.Time
	UpdatedBy string
	DeletedAt *time.Time
	DeletedBy string
}
