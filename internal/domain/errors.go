// Package domain provides pure domain entities, sentinel errors,
// and repository/service interfaces for the Relaybox webhook relay service.
// It must remain free of any external framework/driver imports (RULE-ARCH-01/02).
package domain

import "errors"

// Sentinel errors for domain operations (RULE-ARCH-03: explicit domain error taxonomy).
var (
	ErrNotFound        = errors.New("domain: resource not found")
	ErrConflict        = errors.New("domain: resource conflict")
	ErrInvalidInput    = errors.New("domain: invalid input")
	ErrDuplicateEvent  = errors.New("domain: duplicate event")
	ErrTenantForbidden = errors.New("domain: cross-tenant access forbidden")
)

// IsNotFound returns true when err wraps ErrNotFound.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsConflict returns true when err wraps ErrConflict.
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// IsInvalidInput returns true when err wraps ErrInvalidInput.
func IsInvalidInput(err error) bool { return errors.Is(err, ErrInvalidInput) }
