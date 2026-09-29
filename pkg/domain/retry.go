package domain

import "time"

type RetryPolicy struct {
	PermanentAttempts int
	Base, Cap         time.Duration
}

func (p RetryPolicy) Decide(now time.Time, status, attempts int) Outcome {
	if status >= 200 && status < 300 {
		return Outcome{State: Delivered}
	}
	limit := p.PermanentAttempts
	if limit <= 0 {
		limit = 5
	}
	if status >= 400 && status < 500 && attempts >= limit {
		return Outcome{State: Deadletter}
	}
	base, capDelay := p.Base, p.Cap
	if base <= 0 {
		base = time.Second
	}
	if capDelay <= 0 {
		capDelay = 5 * time.Minute
	}
	delay := min(base, capDelay)
	for n := 1; n < attempts && delay < capDelay; n++ {
		if delay > capDelay/2 {
			delay = capDelay
		} else {
			delay *= 2
		}
	}
	return Outcome{State: Pending, NextAttemptAt: now.Add(delay)}
}
