// Package domain contains core business entities, sentinel errors, and port interfaces.
// It remains pure — no database drivers, no HTTP frameworks (RULE-ARCH-01).
package domain

import "errors"

// Sentinel domain errors (RULE-ARCH-03).
var (
	ErrNotFound           = errors.New("domain: resource not found")
	ErrConflict           = errors.New("domain: resource conflict")
	ErrInvalidState       = errors.New("domain: invalid state transition")
	ErrTenantMismatch     = errors.New("domain: tenant mismatch — cross-tenant access denied")
	ErrMaxRetriesExceeded = errors.New("domain: max delivery retries exceeded")
)
