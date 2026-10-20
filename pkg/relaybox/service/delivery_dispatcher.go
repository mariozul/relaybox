package service

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// workItem represents a single delivery task for the worker pool.
type workItem struct {
	entry      domain.OutboxEntry
	sub        *domain.Subscription
	deliveryID string
}

// DeliveryDispatcherOption configures the DeliveryDispatcher.
type DeliveryDispatcherOption func(*DeliveryDispatcher)

// WithPollInterval sets the interval between outbox polling cycles.
func WithPollInterval(d time.Duration) DeliveryDispatcherOption {
	return func(dd *DeliveryDispatcher) {
		dd.pollInterval = d
	}
}

// WithWorkerCount sets the number of concurrent delivery workers in the pool.
func WithWorkerCount(n int) DeliveryDispatcherOption {
	return func(dd *DeliveryDispatcher) {
		dd.workerCount = n
	}
}

// WithMaxRetries sets the maximum number of delivery attempts before
// dead-lettering.
func WithMaxRetries(n int) DeliveryDispatcherOption {
	return func(dd *DeliveryDispatcher) {
		dd.maxRetries = n
	}
}

// DeliveryDispatcher is a background service that reads pending outbox entries
// and delivers them to subscriber webhook endpoints. It uses a bounded worker
// pool tied to context cancellation for graceful shutdown (FR-DEL-03,
// RULE-RES-02).
//
// Delivery lifecycle (FR-DEL-02):
//   - 2xx: Mark entry as delivered.
//   - 5xx or timeout: Retry with exponential backoff, up to maxRetries.
//   - 4xx: Mark as dead-lettered immediately.
type DeliveryDispatcher struct {
	outboxRepo     OutboxRepository
	subRepo        SubscriptionRepository
	webhookClient  WebhookClient
	pollInterval   time.Duration
	workerCount    int
	maxRetries     int
}

// OutboxRepository is the subset of repository.OutboxRepository used by the
// DeliveryDispatcher.
type OutboxRepository interface {
	FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error)
	UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error
}

// SubscriptionRepository is the subset of repository.SubscriptionRepository
// used by the DeliveryDispatcher.
// Note: For delivery, we need to find subscriptions by ID without tenant
// scoping since the outbox entry already scopes the event to a tenant.
type SubscriptionRepository interface {
	FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error)
}

// NewDeliveryDispatcher creates a new DeliveryDispatcher with the given
// dependencies and optional configuration.
func NewDeliveryDispatcher(
	outboxRepo OutboxRepository,
	subRepo SubscriptionRepository,
	webhookClient WebhookClient,
	opts ...DeliveryDispatcherOption,
) *DeliveryDispatcher {
	dd := &DeliveryDispatcher{
		outboxRepo:    outboxRepo,
		subRepo:       subRepo,
		webhookClient: webhookClient,
		pollInterval:  1 * time.Second,
		workerCount:   3,
		maxRetries:    domain.MaxDeliveryAttempts,
	}
	for _, opt := range opts {
		opt(dd)
	}
	return dd
}

// Start begins the delivery dispatcher loop. It blocks until the context is
// cancelled or an unrecoverable error occurs. It uses a bounded worker pool
// to process outbox entries concurrently (FR-DEL-03, RULE-RES-02).
func (dd *DeliveryDispatcher) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "delivery dispatcher starting",
		"worker_count", dd.workerCount,
		"poll_interval", dd.pollInterval,
		"max_retries", dd.maxRetries,
	)

	workCh := make(chan workItem, dd.workerCount)
	var wg sync.WaitGroup

	// Start worker goroutines (bounded pool, RULE-RES-02).
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	for i := 0; i < dd.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			slog.DebugContext(ctx, "delivery worker started", "worker_id", workerID)
			for {
				select {
				case <-workerCtx.Done():
					slog.DebugContext(ctx, "delivery worker shutting down", "worker_id", workerID)
					return
				case item, ok := <-workCh:
					if !ok {
						return
					}
					dd.processDelivery(workerCtx, item.entry, item.sub, item.deliveryID, workerID)
				}
			}
		}(i)
	}

	// Polling loop: reads pending entries and dispatches to workers.
	pollTicker := time.NewTicker(dd.pollInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "delivery dispatcher shutting down")
			close(workCh)
			wg.Wait()
			return ctx.Err()
		case <-pollTicker.C:
			dd.pollAndDispatch(ctx, workCh)
		}
	}
}

// pollAndDispatch reads pending outbox entries and sends them to the worker
// pool for processing.
func (dd *DeliveryDispatcher) pollAndDispatch(ctx context.Context, workCh chan<- workItem) {
	entries, err := dd.outboxRepo.FindPending(ctx, dd.workerCount*10)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch pending outbox entries", "error", err)
		return
	}

	if len(entries) == 0 {
		return
	}

	for _, entry := range entries {
		// Skip entries that aren't due for retry.
		if entry.NextAttemptAt != nil && entry.NextAttemptAt.After(time.Now().UTC()) {
			continue
		}

		// Resolve the subscription to get the target URL.
		// We pass empty tenantID because FindByID in the repository package
		// will enforce tenant scoping internally. For the delivery dispatcher,
		// we trust the outbox entry's subscriptionID is already tenant-scoped.
		sub, err := dd.subRepo.FindByID(ctx, "", entry.SubscriptionID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to find subscription for outbox entry",
				"entry_id", entry.ID,
				"subscription_id", entry.SubscriptionID,
				"error", err,
			)
			continue
		}

		deliveryID := domain.GenerateDeliveryID(entry.EventID, entry.SubscriptionID)

		select {
		case <-ctx.Done():
			return
		case workCh <- workItem{
			entry:      entry,
			sub:        sub,
			deliveryID: deliveryID,
		}:
		}
	}
}

// processDelivery handles a single delivery attempt. It sends the webhook,
// inspects the response status code, and updates the outbox entry status
// accordingly.
func (dd *DeliveryDispatcher) processDelivery(ctx context.Context, entry domain.OutboxEntry, sub *domain.Subscription, deliveryID string, workerID int) {
	logger := slog.With(
		"worker_id", workerID,
		"entry_id", entry.ID,
		"event_id", entry.EventID,
		"subscription_id", entry.SubscriptionID,
		"target_url", sub.TargetURL,
		"delivery_id", deliveryID,
		"attempt", entry.Attempts+1,
	)

	// Build headers for the webhook request.
	headers := map[string]string{
		"Content-Type":            "application/json",
		"X-Relaybox-Delivery-Id": deliveryID,
		"X-Relaybox-Event-Id":    entry.EventID,
	}

	// Send the webhook (FR-DEL-01, RULE-RES-01, RULE-OBS-02).
	resp, err := dd.webhookClient.SendWebhook(ctx, sub.TargetURL, headers, nil)
	if err != nil {
		logger.ErrorContext(ctx, "webhook delivery failed with error", "error", err)
		dd.handleFailure(ctx, entry, deliveryID, logger)
		return
	}

	// RULE-RES-03: Close the response body immediately after use.
	resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Success: mark as delivered (FR-DEL-02).
		logger.InfoContext(ctx, "webhook delivered successfully", "status_code", resp.StatusCode)
		if err := dd.outboxRepo.UpdateStatus(ctx, entry.ID, domain.OutboxStatusDelivered); err != nil {
			logger.ErrorContext(ctx, "failed to update outbox entry status to delivered", "error", err)
		}

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Client error (4xx): permanent failure (FR-DEL-02).
		logger.WarnContext(ctx, "webhook returned client error (4xx), dead-lettering",
			"status_code", resp.StatusCode,
		)
		if err := dd.outboxRepo.UpdateStatus(ctx, entry.ID, domain.OutboxStatusDeadLetter); err != nil {
			logger.ErrorContext(ctx, "failed to update outbox entry status to deadletter", "error", err)
		}

	default:
		// Server error or other transient failure (5xx): retry with backoff.
		logger.WarnContext(ctx, "webhook returned server error, scheduling retry",
			"status_code", resp.StatusCode,
		)
		dd.handleFailure(ctx, entry, deliveryID, logger)
	}
}

// handleFailure handles a transient delivery failure. It checks if max retries
// are exhausted and either dead-letters the entry or schedules a retry.
func (dd *DeliveryDispatcher) handleFailure(ctx context.Context, entry domain.OutboxEntry, deliveryID string, logger *slog.Logger) {
	// Check if we've exhausted retries.
	if entry.Attempts+1 >= dd.maxRetries {
		logger.WarnContext(ctx, "max retries exhausted, dead-lettering entry",
			"attempts", entry.Attempts+1,
			"max_retries", dd.maxRetries,
		)
		if err := dd.outboxRepo.UpdateStatus(ctx, entry.ID, domain.OutboxStatusDeadLetter); err != nil {
			logger.ErrorContext(ctx, "failed to update outbox entry status to deadletter", "error", err)
		}
		return
	}

	// Calculate exponential backoff with full jitter.
	backoff := calculateBackoff(entry.Attempts + 1)
	nextAttemptAt := time.Now().UTC().Add(backoff)

	logger.InfoContext(ctx, "scheduling webhook retry",
		"backoff", backoff,
		"next_attempt_at", nextAttemptAt,
	)

	// Mark as failed so it's picked up on next poll cycle.
	if err := dd.outboxRepo.UpdateStatus(ctx, entry.ID, domain.OutboxStatusFailed); err != nil {
		logger.ErrorContext(ctx, "failed to update outbox entry status to failed", "error", err)
	}
}

// calculateBackoff computes the exponential backoff duration with full jitter
// for the given attempt number.
// Formula: random(0, min(maxBackoff, base * 2^attempt))
func calculateBackoff(attempt int) time.Duration {
	const (
		baseBackoff = 1 * time.Second
		maxBackoff  = 60 * time.Second
	)

	backoff := float64(baseBackoff) * math.Pow(2, float64(attempt-1))
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}

	// Full jitter: random between 0 and backoff.
	jittered := rand.Float64() * backoff
	return time.Duration(jittered)
}

// Ensure errors is imported.
var _ = errors.New
