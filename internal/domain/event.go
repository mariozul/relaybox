package domain

import "time"

// Event represents an ingested domain event scoped to a tenant.
// Audit column CreatedBy tracks the mutation actor (RULE-DATA-03).
type Event struct {
	ID        string
	TenantID  string
	EventType string
	Payload   []byte
	DedupKey  string
	CreatedAt time.Time
	CreatedBy string
}
