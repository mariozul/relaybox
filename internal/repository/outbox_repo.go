package repository

import (
	"context"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

// OutboxRepository defines persistence operations for the transactional outbox table.
type OutboxRepository interface {
	// Create inserts an outbox entry. Called within the same transaction as
	// event creation (RULE-DATA-01, RULE-EVT-02).
	Create(ctx context.Context, entry *domain.OutboxEntry) error

	// ClaimPending locks and returns up to `limit` pending outbox rows whose
	// NextAttemptAt <= now. Uses row-level locking (FOR UPDATE SKIP LOCKED)
	// to enable concurrent claim safety across worker goroutines.
	ClaimPending(ctx context.Context, now time.Time, limit int) ([]domain.OutboxEntry, error)

	// MarkDelivered marks an outbox entry as successfully delivered.
	MarkDelivered(ctx context.Context, id string) error

	// MarkFailed updates the attempt count, next attempt time, and error message
	// for a failed delivery. If attempts >= policy max, marks as dead-lettered.
	MarkFailed(ctx context.Context, id string, attempts int, nextAttemptAt time.Time, lastError string) error

	// MarkDeadLetter permanently marks an entry as dead-lettered.
	MarkDeadLetter(ctx context.Context, id string, lastError string) error
}
