package api

// CreateEventRequest is the JSON body for POST /v1/events.
// Tenant identity is NEVER in the body (RULE-SEC-01).
type CreateEventRequest struct {
	EventType string `json:"event_type"`
	Payload   []byte `json:"payload"`
	DedupKey  string `json:"dedup_key"`
}

// CreateEventResponse is the JSON response for event creation.
type CreateEventResponse struct {
	EventID        string `json:"event_id"`
	OutboxRowCount int    `json:"outbox_row_count"`
}

// CreateSubscriptionRequest is the JSON body for POST /v1/subscriptions.
type CreateSubscriptionRequest struct {
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
}

// SubscriptionResponse is the JSON response for a subscription.
type SubscriptionResponse struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
}

// ErrorResponse is a generic API error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
