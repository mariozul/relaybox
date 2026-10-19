// Package service implements application business logic (RULE-ARCH-01).
// The dispatcher is the background worker responsible for reading pending
// outbox entries and delivering them to subscriber endpoints with retry,
// backoff, and dead-lettering (FR-DEL-01, FR-DEL-02).
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/infrastructure/httpclient"
	"github.com/mariozul/relaybox/pkg/relaybox/repository"
	"github.com/mariozul/relaybox/pkg/relaybox/worker"
)

// Constants for delivery behavior.
const (
	// maxDeliveryAttempts is the maximum number of delivery attempts before
	// dead-lettering (FR-DEL-02).
	maxDeliveryAttempts = 3

	// baseBackoffDuration is the initial backoff duration for retries.
	baseBackoffDuration = 1 * time.Second

	// maxBatchSize is the maximum number of outbox entries to fetch per poll cycle.
	maxBatchSize = 50

	// pollInterval is the interval between outbox polling cycles.
	pollInterval = 1 * time.Second
)

// Dispatcher defines the contract for the background event delivery worker
// (FR-DEL-01, FR-DEL-03).
type Dispatcher interface {
	// Start begins the dispatcher loop. It blocks until ctx is cancelled.
	// Should be run in a goroutine (FR-DEL-03, RULE-RES-02).
	Start(ctx context.Context)

	// Stop terminates the dispatcher and its worker pool gracefully.
	Stop()
}

// dispatcher implements the Dispatcher interface.
type dispatcher struct {
	outboxRepo repository.OutboxRepository
	subRepo    repository.SubscriptionRepository
	eventRepo  repository.EventRepository
	httpClient httpclient.Client
	workerPool worker.WorkerPool
	clock      func() time.Time
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(
	outboxRepo repository.OutboxRepository,
	subRepo repository.SubscriptionRepository,
	eventRepo repository.EventRepository,
	httpClient httpclient.Client,
	pool worker.WorkerPool,
	clock func() time.Time,
) Dispatcher {
	return &dispatcher{
		outboxRepo: outboxRepo,
		subRepo:    subRepo,
		eventRepo:  eventRepo,
		httpClient: httpClient,
		workerPool: pool,
		clock:      clock,
	}
}

// Start runs the dispatcher loop. It periodically polls the outbox for
// pending entries and dispatches them via the worker pool.
//
// The method blocks until ctx is cancelled, making it suitable for running
// in a goroutine (FR-DEL-03, RULE-RES-02).
func (d *dispatcher) Start(ctx context.Context) {
	slog.InfoContext(ctx, "dispatcher started")

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "dispatcher context cancelled, stopping")
			d.Stop()
			return
		case <-ticker.C:
			d.processPending(ctx)
		}
	}
}

// Stop terminates the dispatcher's worker pool gracefully.
func (d *dispatcher) Stop() {
	d.workerPool.Stop()
}

// processPending fetches pending outbox entries and dispatches them for
// delivery via the worker pool.
func (d *dispatcher) processPending(ctx context.Context) {
	entries, err := d.outboxRepo.GetPendingOutboxEntries(ctx, maxBatchSize)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch pending outbox entries", "err", err)
		return
	}

	for _, entry := range entries {
		entry := entry // Capture for closure
		err := d.workerPool.Submit(func(workerCtx context.Context) {
			d.deliverEntry(workerCtx, &entry)
		})
		if err != nil {
			slog.WarnContext(ctx, "failed to submit outbox entry to worker pool",
				"entry_id", entry.ID, "err", err)
		}
	}
}

// deliverEntry handles delivery of a single outbox entry to its subscriber.
func (d *dispatcher) deliverEntry(ctx context.Context, entry *domain.OutboxEntry) {
	logger := slog.With(
		"entry_id", entry.ID,
		"event_id", entry.EventID,
		"subscription_id", entry.SubscriptionID,
		"delivery_id", entry.DeliveryID,
		"attempt", entry.Attempts+1,
	)

	// Resolve the subscription to get the target URL and tenant context.
	sub, err := d.resolveSubscription(ctx, entry)
	if err != nil {
		logger.ErrorContext(ctx, "failed to resolve subscription for outbox entry", "err", err)
		d.markDeadLetter(ctx, entry, fmt.Sprintf("subscription resolution failed: %v", err))
		return
	}

	// Fetch the event payload to include in the delivery body.
	evt, err := d.eventRepo.GetEventByID(ctx, entry.EventID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to fetch event for outbox entry", "err", err)
		d.markDeadLetter(ctx, entry, fmt.Sprintf("event fetch failed: %v", err))
		return
	}
	if evt == nil {
		logger.ErrorContext(ctx, "event not found for outbox entry")
		d.markDeadLetter(ctx, entry, "event not found")
		return
	}

	// Build the delivery payload: wrap the event payload with metadata.
	body, err := d.buildDeliveryPayload(evt, entry)
	if err != nil {
		logger.ErrorContext(ctx, "failed to build delivery payload", "err", err)
		d.markDeadLetter(ctx, entry, fmt.Sprintf("build payload failed: %v", err))
		return
	}

	// Send the HTTP POST with delivery headers (FR-DEL-01, FR-DEL-04).
	headers := map[string]string{
		"X-Relaybox-Delivery-Id": entry.DeliveryID,
		"Content-Type":           "application/json",
	}

	resp, err := d.httpClient.Post(ctx, sub.TargetURL, headers, body)
	if err != nil {
		logger.WarnContext(ctx, "delivery attempt failed (network error)", "err", err)
		d.handleDeliveryFailure(ctx, entry, fmt.Sprintf("network error: %v", err))
		return
	}
	defer httpclient.EnsureResponseBodyClosed(resp)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success (2xx): mark as delivered (FR-DEL-02).
		d.markDelivered(ctx, entry)
		logger.InfoContext(ctx, "event delivered successfully")
	} else {
		// Non-2xx response — retry with backoff; dead-letter after max attempts
		// (FR-DEL-02: 4xx is dead-lettered after N attempts, 5xx retried).
		errStr := fmt.Sprintf("subscriber returned %d", resp.StatusCode)
		logger.WarnContext(ctx, "delivery failure (non-2xx)", "status", resp.StatusCode)
		d.handleDeliveryFailure(ctx, entry, errStr)
	}
}

// buildDeliveryPayload creates the JSON body for the webhook delivery,
// including the original event payload and metadata.
func (d *dispatcher) buildDeliveryPayload(evt *domain.Event, entry *domain.OutboxEntry) ([]byte, error) {
	payload := map[string]interface{}{
		"event_id":     evt.ID,
		"event_type":   evt.EventType,
		"payload":      json.RawMessage(evt.Payload),
		"delivery_id":  entry.DeliveryID,
		"attempt":      entry.Attempts + 1,
	}
	return json.Marshal(payload)
}

// resolveSubscription looks up the subscription for the given outbox entry.
// The entry's subscription_id must exist and be accessible.
func (d *dispatcher) resolveSubscription(ctx context.Context, entry *domain.OutboxEntry) (*domain.Subscription, error) {
	if entry.SubscriptionID == "" {
		return nil, fmt.Errorf("outbox entry %s has no subscription ID", entry.ID)
	}

	// Fetch subscription without tenant scope — the outbox entry already
	// associates the subscription. The subscription itself is tenant-scoped
	// in the repository.
	sub, err := d.subRepo.GetSubscriptionByID(ctx, "", entry.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("get subscription %s: %w", entry.SubscriptionID, err)
	}
	if sub == nil {
		return nil, fmt.Errorf("subscription %s not found", entry.SubscriptionID)
	}
	return sub, nil
}

// handleDeliveryFailure handles a failed delivery attempt. If the maximum
// number of attempts has been reached, the entry is dead-lettered. Otherwise,
// it's scheduled for retry with exponential backoff (FR-DEL-02).
func (d *dispatcher) handleDeliveryFailure(ctx context.Context, entry *domain.OutboxEntry, errStr string) {
	newAttempts := entry.Attempts + 1

	if newAttempts >= maxDeliveryAttempts {
		d.markDeadLetter(ctx, entry, errStr)
		return
	}

	// Calculate exponential backoff: base * 2^(attempts-1)
	backoff := time.Duration(float64(baseBackoffDuration) * math.Pow(2, float64(newAttempts-1)))
	nextAttempt := d.clock().Add(backoff)

	err := d.outboxRepo.MarkOutboxEntryProcessed(ctx, entry.ID, "failed", newAttempts, &nextAttempt, &errStr)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update outbox entry after transient failure",
			"entry_id", entry.ID, "err", err)
	}
}

// markDelivered marks an outbox entry as successfully delivered.
func (d *dispatcher) markDelivered(ctx context.Context, entry *domain.OutboxEntry) {
	err := d.outboxRepo.MarkOutboxEntryProcessed(
		ctx, entry.ID, "delivered", entry.Attempts+1, nil, nil,
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to mark outbox entry as delivered",
			"entry_id", entry.ID, "err", err)
	}
}

// markDeadLetter marks an outbox entry as dead-lettered (permanently failed).
func (d *dispatcher) markDeadLetter(ctx context.Context, entry *domain.OutboxEntry, errStr string) {
	err := d.outboxRepo.MarkOutboxEntryProcessed(
		ctx, entry.ID, "dead_letter", entry.Attempts+1, nil, &errStr,
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to mark outbox entry as dead_letter",
			"entry_id", entry.ID, "err", err)
	}
}
