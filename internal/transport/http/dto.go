package http

import "encoding/json"

// IngestEventRequest is the JSON body for POST /v1/events.
type IngestEventRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	DedupKey  string          `json:"dedup_key"`
}

// IngestEventResponse is returned from successful event ingestion.
type IngestEventResponse struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	EventType string `json:"event_type"`
	DedupKey  string `json:"dedup_key"`
}

// CreateSubscriptionRequest is the JSON body for POST /v1/subscriptions.
type CreateSubscriptionRequest struct {
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
}

// SubscriptionResponse is returned for subscription operations.
type SubscriptionResponse struct {
	ID        string  `json:"id"`
	TenantID  string  `json:"tenant_id"`
	EventType string  `json:"event_type"`
	TargetURL string  `json:"target_url"`
	CreatedAt string  `json:"created_at"`
	CreatedBy string  `json:"created_by"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	DeletedAt *string `json:"deleted_at,omitempty"`
}

// ErrorResponse is a standard error payload.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
}
