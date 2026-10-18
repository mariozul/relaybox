package repository

import (
	"context"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// OutboxRepository defines the contract for transactional outbox persistence.
type OutboxRepository interface {
	// EnqueueEvent adds a new entry to the outbox for delivery.
	EnqueueEvent(ctx context.Context, entry *domain.OutboxEntry) (string, error)

	// GetPendingOutboxEntries retrieves entries ready for delivery, limited by count.
	GetPendingOutboxEntries(ctx context.Context, limit int) ([]domain.OutboxEntry, error)

	// MarkOutboxEntryProcessed updates the status and attempt info of an outbox entry.
	MarkOutboxEntryProcessed(ctx context.Context, id string, status string, attempts int, nextAttemptAt *time.Time, lastAttemptError *string) error
}

// TransactionManager provides an interface for managing database transactions (RULE-DATA-01).
type TransactionManager interface {
	// BeginTx starts a new transaction.
	BeginTx(ctx context.Context) (interface{}, error)
	// CommitTx commits a transaction.
	CommitTx(ctx context.Context, tx interface{}) error
	// RollbackTx rolls back a transaction.
	RollbackTx(ctx context.Context, tx interface{}) error
	// ExecInTx executes a function within a transaction with automatic rollback on error.
	ExecInTx(ctx context.Context, fn func(txCtx context.Context, tx interface{}) error) error
}
