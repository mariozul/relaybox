package domain

import "time"

// Clock is an injectable interface for obtaining the current time.
// Domain logic MUST use Clock.Now() instead of time.Now() to enable
// deterministic testing and avoid hard-to-test time dependencies.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}
