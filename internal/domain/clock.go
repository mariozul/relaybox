package domain

import "time"

// Clock abstracts time operations to enable deterministic testing.
type Clock interface {
	Now() time.Time
}

// RealClock returns the current system time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
