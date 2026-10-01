package domain

import "errors"

var (
	ErrEventDuplicate       = errors.New("event duplicate")
	ErrInvalidEventPayload  = errors.New("invalid event payload")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrTenantMissing        = errors.New("tenant missing")
	ErrNotFound             = errors.New("not found")
)
