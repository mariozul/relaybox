package domain

import "errors"

// Sentinel domain errors (RULE-ARCH-03).
var (
	ErrEventAlreadyExists   = errors.New("event already exists for this tenant with the same dedup key")
	ErrSubscriptionNotFound = errors.New("subscription not found or access denied")
	ErrUnauthorized         = errors.New("unauthorized access to resource")
	ErrInvalidInput         = errors.New("invalid input provided")
	ErrInternal             = errors.New("internal server error")
)

// Error codes for protocol mapping.
const (
	ErrCodeEventAlreadyExists   = "EVENT_ALREADY_EXISTS"
	ErrCodeSubscriptionNotFound = "SUBSCRIPTION_NOT_FOUND"
	ErrCodeUnauthorized         = "UNAUTHORIZED_ACCESS"
	ErrCodeInvalidInput         = "INVALID_INPUT"
	ErrCodeInternal             = "INTERNAL_SERVER_ERROR"
)
