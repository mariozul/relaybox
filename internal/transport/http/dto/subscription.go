package dto

type CreateSubscriptionRequest struct {
	EventType string `json:"event_type"`
	TargetURL string `json:"target_url"`
}

type SubscriptionResponse struct {
	ID        string  `json:"id"`
	TenantID  string  `json:"tenant_id"`
	EventType string  `json:"event_type"`
	TargetURL string  `json:"target_url"`
	CreatedAt string  `json:"created_at"`
	CreatedBy string  `json:"created_by"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	UpdatedBy *string `json:"updated_by,omitempty"`
	DeletedAt *string `json:"deleted_at,omitempty"`
	DeletedBy *string `json:"deleted_by,omitempty"`
}
