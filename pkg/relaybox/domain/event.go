package domain

import "time"

// Event represents a domain event ingested into the system.
type Event struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	EventType string    `json:"event_type"`
	Payload   []byte    `json:"payload"`
	DedupKey  string    `json:"dedup_key"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}
