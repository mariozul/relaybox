package domain

import (
	"math"
	"time"
)

// DeliveryPolicy configures retry semantics for outbox delivery.
type DeliveryPolicy struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	MaxRetries int
}

// DefaultDeliveryPolicy returns the canonical retry configuration:
// 1s base, 2x multiplier, 60s cap, 8 max attempts.
func DefaultDeliveryPolicy() DeliveryPolicy {
	return DeliveryPolicy{
		BaseDelay:  1 * time.Second,
		MaxDelay:   60 * time.Second,
		MaxRetries: 8,
	}
}

// NextBackoff computes the exponential backoff delay for the given attempt
// (1-indexed). The formula is base * 2^(attempt-1), capped at MaxDelay.
func (p DeliveryPolicy) NextBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	backoff := float64(p.BaseDelay) * math.Pow(2, float64(attempt-1))
	dur := time.Duration(backoff)
	if dur > p.MaxDelay {
		dur = p.MaxDelay
	}
	return dur
}

// IsExhausted returns true when the attempt count has reached MaxRetries.
func (p DeliveryPolicy) IsExhausted(attempts int) bool {
	return attempts >= p.MaxRetries
}
