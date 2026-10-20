// Package domain defines the core business entities, value objects, and error
// types for the Relaybox multi-tenant webhook relay service. It maintains a
// pure domain layer free from infrastructure concerns (RULE-ARCH-01).
package domain

import "fmt"

// DomainError represents a typed domain error with a machine-readable code
// and a human-readable message. It implements the error interface via Error()
// and supports sentinel error matching via Is().
type DomainError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error returns the string representation of the DomainError.
func (e *DomainError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Is enables error matching against sentinel pointers.
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// sentinel domain errors (RULE-ARCH-03).
var (
	// ErrInvalidEventPayload is returned when the event payload is malformed
	// or contains invalid/missing required fields.
	ErrInvalidEventPayload = &DomainError{
		Code:    "RELAYBOX_INVALID_EVENT_PAYLOAD",
		Message: "the provided event payload is malformed or invalid",
	}

	// ErrDuplicateEvent is returned when an event with the same dedup_key
	// already exists for this tenant.
	ErrDuplicateEvent = &DomainError{
		Code:    "RELAYBOX_DUPLICATE_EVENT",
		Message: "an event with the same dedup_key already exists for this tenant",
	}

	// ErrSubscriptionNotFound is returned when the requested subscription
	// could not be found.
	ErrSubscriptionNotFound = &DomainError{
		Code:    "RELAYBOX_SUBSCRIPTION_NOT_FOUND",
		Message: "the requested subscription could not be found",
	}

	// ErrUnauthorized is returned when the authenticated tenant lacks
	// permission for the requested resource.
	ErrUnauthorized = &DomainError{
		Code:    "RELAYBOX_UNAUTHORIZED",
		Message: "the authenticated tenant lacks permission for this resource",
	}

	// ErrInternal is returned when an unexpected internal server error occurs.
	ErrInternal = &DomainError{
		Code:    "RELAYBOX_INTERNAL_ERROR",
		Message: "an unexpected internal server error occurred",
	}
)
