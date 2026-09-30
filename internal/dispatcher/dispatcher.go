package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

type Config struct {
	Workers, BatchSize, MaxAttempts             int
	PollInterval, RequestTimeout, LeaseDuration time.Duration
}

type Dispatcher struct {
	store  domain.Store
	sender domain.Sender
	config Config
	logger *slog.Logger
}

func New(store domain.Store, sender domain.Sender, config Config) *Dispatcher {
	if config.Workers < 1 {
		config.Workers = 4
	}
	if config.BatchSize < 1 {
		config.BatchSize = 20
	}
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 5
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 10 * time.Second
	}
	if config.LeaseDuration <= 0 {
		config.LeaseDuration = 30 * time.Second
	}
	return &Dispatcher{store: store, sender: sender, config: config, logger: slog.Default()}
}

func (d *Dispatcher) Run(ctx context.Context) error {
	jobs := make(chan domain.Delivery, d.config.BatchSize)
	var workers sync.WaitGroup
	for range d.config.Workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for delivery := range jobs {
				d.process(ctx, delivery)
			}
		}()
	}
	defer func() { close(jobs); workers.Wait() }()
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()
	for {
		deliveries, err := d.store.Claim(ctx, time.Now().UTC(), d.config.BatchSize, d.config.LeaseDuration)
		if err != nil {
			return fmt.Errorf("claim deliveries: %w", err)
		}
		for _, delivery := range deliveries {
			select {
			case jobs <- delivery:
			case <-ctx.Done():
				return nil
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (d *Dispatcher) process(ctx context.Context, delivery domain.Delivery) {
	requestCtx, cancel := context.WithTimeout(ctx, d.config.RequestTimeout)
	status, err := d.sender.Send(requestCtx, delivery)
	cancel()
	now := time.Now().UTC()
	if err == nil && status >= 200 && status < 300 {
		if markErr := d.store.MarkDelivered(ctx, delivery, now); markErr != nil {
			d.logger.ErrorContext(ctx, "mark delivery", "error", markErr, "delivery_id", delivery.ID)
		}
		return
	}
	dead := delivery.Attempts+1 >= d.config.MaxAttempts
	reason := fmt.Sprintf("http status %d", status)
	if err != nil {
		reason = err.Error()
	}
	next := now.Add(time.Second * time.Duration(math.Pow(2, float64(delivery.Attempts))))
	if markErr := d.store.MarkFailed(ctx, delivery, next, reason, dead); markErr != nil {
		d.logger.ErrorContext(ctx, "mark failure", "error", markErr, "delivery_id", delivery.ID)
	}
}
