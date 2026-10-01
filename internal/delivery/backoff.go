package delivery

import (
	"math"
	"math/rand"
	"time"
)

const (
	defaultBaseDelay = 1 * time.Second
	maxDelay         = 5 * time.Minute
	maxAttempts      = 5
)

// Backoff computes the next delay with exponential backoff and jitter.
// Returns true if the attempt should be retried, false if max attempts reached.
func Backoff(attempts int) (time.Duration, bool) {
	if attempts >= maxAttempts {
		return 0, false
	}
	// Exponential: base * 2^attempts with 20% jitter.
	base := float64(defaultBaseDelay) * math.Pow(2, float64(attempts))
	jitter := base * 0.2 * (rand.Float64()*2 - 1)
	delay := time.Duration(base + jitter)
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay, true
}

// MaxAttempts returns the configured max delivery attempts before dead-letter.
func MaxAttempts() int { return maxAttempts }
