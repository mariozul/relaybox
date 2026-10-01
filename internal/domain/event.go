package domain

import "time"

type Event struct {
	ID        string
	TenantID  string
	EventType string
	Payload   []byte
	DedupKey  string
	CreatedAt time.Time
	CreatedBy string
}

type Subscription struct {
	ID        string
	TenantID  string
	EventType string
	TargetURL string
	CreatedAt time.Time
	CreatedBy string
	UpdatedAt *time.Time
	UpdatedBy *string
	DeletedAt *time.Time
	DeletedBy *string
}

type DeliveryStatus string

const (
	DeliveryStatusPending    DeliveryStatus = "pending"
	DeliveryStatusDelivered  DeliveryStatus = "delivered"
	DeliveryStatusDeadLetter DeliveryStatus = "dead_letter"
)

type OutboxEntry struct {
	ID             string
	EventID        string
	SubscriptionID string
	Status         DeliveryStatus
	Attempts       int
	NextAttemptAt  time.Time
	LastAttemptAt  *time.Time
	DeliveryID     string
	CreatedAt      time.Time
	CreatedBy      string
}
