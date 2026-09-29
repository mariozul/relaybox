package domain

import (
	"strings"
	"time"
	"unicode"
)

type Identity struct{ TenantID, ActorID string }

func (i Identity) Validate() error {
	for _, value := range []string{i.TenantID, i.ActorID} {
		if value == "" || len(value) > 256 || strings.TrimSpace(value) != value || strings.ContainsFunc(value, unicode.IsControl) {
			return ErrUnauthorized
		}
	}
	return nil
}

type Audit struct {
	CreatedAt, UpdatedAt            time.Time
	DeletedAt                       *time.Time
	CreatedBy, UpdatedBy, DeletedBy string
}
type EventInput struct {
	EventType, DedupKey string
	Payload             []byte
}
type Event struct {
	ID, TenantID string
	EventInput
	Audit Audit
}
type SubscriptionInput struct{ EventType, TargetURL string }
type Subscription struct {
	ID, TenantID string
	SubscriptionInput
	Audit Audit
}
type State string

const (
	Pending    State = "pending"
	Processing State = "processing"
	Delivered  State = "delivered"
	Deadletter State = "deadletter"
)

type Delivery struct {
	ID, TenantID, EventID, SubscriptionID, TargetURL string
	Payload                                          []byte
	TraceParent, TraceState                          string
	State                                            State
	Attempts                                         int
	LeaseToken                                       string
	LeaseUntil, NextAttemptAt                        time.Time
	Audit                                            Audit
}
type Outcome struct {
	State         State
	NextAttemptAt time.Time
}
