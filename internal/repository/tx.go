package repository

import "context"

// TxManager abstracts transaction lifecycle across repositories.
// Implementations wrap database-specific transaction handles (e.g., pgx.Tx).
type TxManager interface {
	// WithTx executes fn inside a single database transaction.
	// If fn returns an error the transaction is rolled back; otherwise committed.
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
