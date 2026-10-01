package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mariozul/relaybox/internal/domain"
)

// TxManager implements domain.TxManager using pgx transactions (RULE-DATA-01).
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a new TxManager.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// RunInTx executes fn in a single database transaction.
func (tm *TxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(ctx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// Pool returns the underlying *pgxpool.Pool.
func (tm *TxManager) Pool() *pgxpool.Pool {
	return tm.pool
}

// Ensure interface implementation.
var _ domain.TxManager = (*TxManager)(nil)
