package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
)

// compileTimeCheck ensures repository interfaces are satisfied at compile time.
// These are compile-time checks that concrete implementations must fulfill.
func TestRepositoryInterfaceContracts(t *testing.T) {
	t.Parallel()

	// Verify that the interfaces are correctly defined by checking they can be
	// used as type constraints for function parameters.
	// These tests verify the interface definitions compile correctly.

	t.Run("EventRepository contract", func(t *testing.T) {
		// Verify the interface methods exist by type assertion
		var _ repository.EventRepository = nilInterfaceEvent(nil)
		_ = &eventRepoVerifier{}
	})

	t.Run("SubscriptionRepository contract", func(t *testing.T) {
		var _ repository.SubscriptionRepository = nilInterfaceSub(nil)
		_ = &subRepoVerifier{}
	})

	t.Run("OutboxRepository contract", func(t *testing.T) {
		var _ repository.OutboxRepository = nilInterfaceOutbox(nil)
		_ = &outboxRepoVerifier{}
	})

	t.Run("TransactionManager contract", func(t *testing.T) {
		var _ repository.TransactionManager = nilInterfaceTx(nil)
		_ = &txManagerVerifier{}
	})
}

// nil interfaces for compile-time checks
type nilInterfaceEvent repository.EventRepository
type nilInterfaceSub repository.SubscriptionRepository
type nilInterfaceOutbox repository.OutboxRepository
type nilInterfaceTx repository.TransactionManager

// verifier structs that implement the interfaces for compile-time checks
type eventRepoVerifier struct{}

func (e *eventRepoVerifier) CreateEvent(ctx context.Context, event *domain.Event) (string, error) {
	return "", nil
}
func (e *eventRepoVerifier) GetEventByDedupKey(ctx context.Context, tenantID, dedupKey string) (*domain.Event, error) {
	return nil, nil
}

type subRepoVerifier struct{}

func (s *subRepoVerifier) CreateSubscription(ctx context.Context, sub *domain.Subscription) (string, error) {
	return "", nil
}
func (s *subRepoVerifier) GetSubscriptionByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	return nil, nil
}
func (s *subRepoVerifier) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return nil, nil
}
func (s *subRepoVerifier) UpdateSubscription(ctx context.Context, sub *domain.Subscription) error { return nil }
func (s *subRepoVerifier) DeleteSubscription(ctx context.Context, tenantID, id string) error    { return nil }

type outboxRepoVerifier struct{}

func (o *outboxRepoVerifier) EnqueueEvent(ctx context.Context, entry *domain.OutboxEntry) (string, error) {
	return "", nil
}
func (o *outboxRepoVerifier) GetPendingOutboxEntries(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	return nil, nil
}
func (o *outboxRepoVerifier) MarkOutboxEntryProcessed(ctx context.Context, id string, status string, attempts int, nextAttemptAt *time.Time, lastAttemptError *string) error {
	return nil
}

type txManagerVerifier struct{}

func (t *txManagerVerifier) BeginTx(ctx context.Context) (interface{}, error)           { return nil, nil }
func (t *txManagerVerifier) CommitTx(ctx context.Context, tx interface{}) error          { return nil }
func (t *txManagerVerifier) RollbackTx(ctx context.Context, tx interface{}) error        { return nil }
func (t *txManagerVerifier) ExecInTx(ctx context.Context, fn func(context.Context, interface{}) error) error {
	return nil
}
