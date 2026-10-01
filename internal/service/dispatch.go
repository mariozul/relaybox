package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
	"github.com/mariozul/relaybox/internal/infrastructure/httpclient"
	"github.com/mariozul/relaybox/internal/repository"
)

type Dispatcher struct {
	outbox   repository.OutboxRepository
	subRepo  repository.SubscriptionRepository
	client   *httpclient.Client
	policy   domain.DefaultDeliveryPolicy
	workers  int
	poll     time.Duration
	logger   *slog.Logger
}

func NewDispatcher(
	outbox repository.OutboxRepository,
	subRepo repository.SubscriptionRepository,
	client *httpclient.Client,
	policy domain.DefaultDeliveryPolicy,
	workers int,
	poll time.Duration,
) *Dispatcher {
	return &Dispatcher{
		outbox:  outbox,
		subRepo: subRepo,
		client:  client,
		policy:   policy,
		workers: workers,
		poll:    poll,
	}
}

// Run starts the bounded worker pool and blocks until ctx is cancelled.
// RULE-RES-02: bounded pool tied to ctx cancellation + graceful WaitGroup shutdown.
func (d *Dispatcher) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i < d.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			d.workerLoop(ctx, workerID)
		}(i)
	}
	wg.Wait()
	return ctx.Err()
}

func (d *Dispatcher) workerLoop(ctx context.Context, id int) {
	ticker := time.NewTicker(d.poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.processBatch(ctx)
		}
	}
}

func (d *Dispatcher) processBatch(ctx context.Context) {
	entries, err := d.outbox.ClaimPending(ctx, 10)
	if err != nil {
		return
	}
	for _, entry := range entries {
		d.processOne(ctx, entry)
	}
}

func (d *Dispatcher) processOne(ctx context.Context, entry *domain.OutboxEntry) {
	// RULE-ARCH-02: dispatch delegates to subscription repo for URL resolution.
	targetURL := d.resolveURL(ctx, entry.SubscriptionID)

	resp, err := d.client.Deliver(ctx, &httpclient.DeliveryRequest{
		URL:        targetURL,
		DeliveryID:  entry.DeliveryID,
		AttemptNum:  entry.Attempts + 1,
	})

	if err != nil {
		d.classifyAndAct(ctx, entry, 0)
		return
	}

	d.classifyAndAct(ctx, entry, resp.StatusCode)
}

func (d *Dispatcher) resolveURL(ctx context.Context, subID string) string {
	if d.subRepo == nil || subID == "" {
		return "" // will error in Deliver; retried
	}
	subs, err := d.subRepo.ListByTenant(ctx, "") // tenant scoping handled separately
	if err != nil || len(subs) == 0 {
		return ""
	}
	for _, s := range subs {
		if s.ID == subID {
			return s.TargetURL
		}
	}
	return ""
}

func (d *Dispatcher) classifyAndAct(ctx context.Context, entry *domain.OutboxEntry, statusCode int) {
	action := d.policy.Classify(&domain.DeliveryAttempt{
		StatusCode: statusCode,
		Attempt:    domain.AttemptInfo{Count: entry.Attempts + 1},
		Now:        time.Now(),
	})

	switch action.Kind {
	case domain.ActionDelivered:
		_ = d.outbox.MarkDelivered(ctx, entry.ID)
	case domain.ActionDeadLetter:
		_ = d.outbox.DeadLetter(ctx, entry.ID)
	case domain.ActionRetry:
		_ = d.outbox.MarkAttempt(ctx, entry.ID, time.Now().Add(action.BackoffDuration))
	}
}
