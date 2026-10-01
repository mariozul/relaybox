package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/repository"
)

// DispatcherConfig configures the outbox dispatcher.
type DispatcherConfig struct {
	PollInterval  time.Duration
	BatchSize     int
	WorkerCount   int
	DeliveryPolicy domain.DeliveryPolicy
}

// DefaultDispatcherConfig returns sensible defaults.
func DefaultDispatcherConfig() DispatcherConfig {
	return DispatcherConfig{
		PollInterval:   2 * time.Second,
		BatchSize:      16,
		WorkerCount:    8,
		DeliveryPolicy: domain.DefaultDeliveryPolicy(),
	}
}

// Dispatcher polls the outbox and delivers events to subscriber endpoints.
type Dispatcher struct {
	config         DispatcherConfig
	outboxRepo     repository.OutboxRepository
	eventRepo      repository.EventRepository
	forwarder      Forwarder
	clock          domain.Clock
	logger         *slog.Logger
	wg             sync.WaitGroup
	cancel         context.CancelFunc
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(
	cfg DispatcherConfig,
	outboxRepo repository.OutboxRepository,
	eventRepo repository.EventRepository,
	forwarder Forwarder,
	clock domain.Clock,
	logger *slog.Logger,
) *Dispatcher {
	return &Dispatcher{
		config:     cfg,
		outboxRepo: outboxRepo,
		eventRepo:  eventRepo,
		forwarder:  forwarder,
		clock:      clock,
		logger:     logger,
	}
}

// Start begins polling and dispatching. It blocks until ctx is cancelled.
// Workers are bounded per RULE-RES-02.
func (d *Dispatcher) Start(ctx context.Context) {
	ctx, d.cancel = context.WithCancel(ctx)
	defer d.cancel()

	sem := make(chan struct{}, d.config.WorkerCount)
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	d.logger.InfoContext(ctx, "dispatcher started",
		"workers", d.config.WorkerCount,
		"poll_interval", d.config.PollInterval,
		"batch_size", d.config.BatchSize,
	)

	for {
		select {
		case <-ctx.Done():
			d.logger.InfoContext(ctx, "dispatcher draining workers")
			d.wg.Wait()
			d.logger.InfoContext(ctx, "dispatcher stopped")
			return
		case <-ticker.C:
			entries, err := d.outboxRepo.ClaimPending(ctx, d.clock.Now(), d.config.BatchSize)
			if err != nil {
				d.logger.ErrorContext(ctx, "dispatcher claim failed", "err", err)
				continue
			}
			for i := range entries {
				entry := entries[i]
				sem <- struct{}{} // acquire worker slot
				d.wg.Add(1)
				go func(e domain.OutboxEntry) {
					defer d.wg.Done()
					defer func() { <-sem }() // release worker slot
					d.processItem(ctx, e)
				}(entry)
			}
		}
	}
}

func (d *Dispatcher) processItem(ctx context.Context, entry domain.OutboxEntry) {
	event, err := d.eventRepo.FindByID(ctx, entry.EventID)
	if err != nil {
		d.logger.ErrorContext(ctx, "dispatcher: event not found",
			"outbox_id", entry.ID,
			"event_id", entry.EventID,
			"err", err,
		)
		return
	}

	result, err := d.forwarder.Forward(ctx, ForwardRequest{
		URL:        "", // Will be populated by adapter from subscription lookup
		Body:       event.Payload,
		DeliveryID: entry.DeliveryID(),
		EventType:  event.EventType,
	})
	if err != nil {
		d.handleFailure(ctx, entry, fmt.Sprintf("forward error: %v", err))
		return
	}

	if result.StatusCode >= 200 && result.StatusCode < 300 {
		if err := d.outboxRepo.MarkDelivered(ctx, entry.ID); err != nil {
			d.logger.ErrorContext(ctx, "dispatcher: mark delivered failed", "outbox_id", entry.ID, "err", err)
		}
		return
	}

	if IsPermanent(result.StatusCode) {
		if err := d.outboxRepo.MarkDeadLetter(ctx, entry.ID, fmt.Sprintf("permanent failure: %d", result.StatusCode)); err != nil {
			d.logger.ErrorContext(ctx, "dispatcher: mark deadletter failed", "outbox_id", entry.ID, "err", err)
		}
		return
	}

	// 5xx or other: retry.
	d.handleFailure(ctx, entry, fmt.Sprintf("http %d", result.StatusCode))
}

func (d *Dispatcher) handleFailure(ctx context.Context, entry domain.OutboxEntry, lastError string) {
	attempts := entry.Attempts + 1
	if d.config.DeliveryPolicy.IsExhausted(attempts) {
		if err := d.outboxRepo.MarkDeadLetter(ctx, entry.ID, lastError); err != nil {
			d.logger.ErrorContext(ctx, "dispatcher: mark deadletter failed", "outbox_id", entry.ID, "err", err)
		}
		return
	}
	nextAttempt := d.clock.Now().Add(d.config.DeliveryPolicy.NextBackoff(attempts))
	if err := d.outboxRepo.MarkFailed(ctx, entry.ID, attempts, nextAttempt, lastError); err != nil {
		d.logger.ErrorContext(ctx, "dispatcher: mark failed failed", "outbox_id", entry.ID, "err", err)
	}
}
