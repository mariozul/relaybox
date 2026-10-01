package domain_test

import (
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

// testClock is a deterministic clock for tests.
type testClock struct {
	t time.Time
}

func (c testClock) Now() time.Time { return c.t }

// Compile-time interface satisfaction check.
var _ domain.Clock = testClock{}
