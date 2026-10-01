package domain

import "errors"

// Sentinel domain errors per RULE-ARCH-03 (Explicit Domain Error Taxonomy).
// Transport handlers map these to appropriate protocol status codes.
var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("entity not found")

	// ErrConflict indicates a conflicting state or constraint violation.
	ErrConflict = errors.New("conflict: state constraint violation")

	// ErrInvalidInput indicates the provided input is malformed or invalid.
	ErrInvalidInput = errors.New("invalid input: required fields missing or malformed")

	// ErrDuplicateEvent indicates an attempt to ingest an event with a
	// duplicate (tenant_id, dedup_key) combination (idempotency guard).
	ErrDuplicateEvent = errors.New("duplicate event: unique constraint violation on (tenant_id, dedup_key)")

	// ErrInvalidState indicates an invalid state transition was attempted
	// (e.g., marking a delivered outbox row back to pending).
	ErrInvalidState = errors.New("invalid state transition")
)
