package domain

import (
	"math"
	"time"
)

// ExponentialBackoff computes the delay for a given attempt using the formula:
//
//	delay = base * factor^(attempt-1)
//
// The result is capped at max. If attempt <= 0, returns 0.
// This is a pure function: no side effects, no time.Sleep, no I/O.
func ExponentialBackoff(base, max time.Duration, factor float64, attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	if attempt == 1 {
		if base > max {
			return max
		}
		return base
	}
	delayNs := float64(base.Nanoseconds()) * math.Pow(factor, float64(attempt-1))
	delay := time.Duration(delayNs)
	if delay <= 0 {
		return max // overflow guard
	}
	if delay > max {
		return max
	}
	return delay
}
