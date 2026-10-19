// Package dto defines data transfer objects (RULE-ARCH-01).
// These are pure data types with no framework dependencies.
package dto

// CreateSubscriptionRequest is the input for creating a subscription.
type CreateSubscriptionRequest struct {
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
}

// UpdateSubscriptionRequest is the input for updating a subscription.
type UpdateSubscriptionRequest struct {
	EventType *string `json:"event_type,omitempty"`
	TargetURL *string `json:"target_url,omitempty"`
}

// SubscriptionResponse is the output representation of a subscription.
type SubscriptionResponse struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	UpdatedAt string `json:"updated_at"`
	UpdatedBy string `json:"updated_by"`
}

// SubscriptionListResponse wraps a list of subscriptions.
type SubscriptionListResponse struct {
	Subscriptions []SubscriptionResponse `json:"subscriptions"`
}
