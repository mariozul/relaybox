// Package domain defines core business entities, sentinels, and contracts
// for the Relaybox webhook relay service. It remains pure Go with zero
// external framework or driver imports (RULE-ARCH-01).
package domain

import "errors"

// Sentinel domain errors mapped by transport handlers to HTTP status codes.
var (
	ErrNotFound            = errors.New("domain: resource not found")
	ErrConflict            = errors.New("domain: resource already exists")
	ErrInvalidArgument     = errors.New("domain: invalid argument")
	ErrInvalidEventPayload = errors.New("domain: invalid event payload")
	ErrTenantRequired      = errors.New("domain: tenant identity required")
)
