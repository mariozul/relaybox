package domain

import (
	"math"
	"time"
)

type ActionKind string

const (
	ActionRetry      ActionKind = "retry"
	ActionDeadLetter ActionKind = "dead_letter"
	ActionDelivered  ActionKind = "delivered"
)

type Action struct {
	Kind            ActionKind
	BackoffDuration time.Duration
}

type DeliveryAttempt struct {
	StatusCode int
	Attempt    AttemptInfo
	Now        time.Time
}

type AttemptInfo struct {
	Count int
}

type DeliveryPolicy interface {
	Classify(attempt *DeliveryAttempt) Action
}

type DefaultDeliveryPolicy struct {
	MaxAttempts int
	BaseBackoff time.Duration
}

func (p DefaultDeliveryPolicy) Classify(attempt *DeliveryAttempt) Action {
	if attempt.StatusCode >= 200 && attempt.StatusCode < 300 {
		return Action{Kind: ActionDelivered}
	}
	if attempt.StatusCode >= 400 && attempt.StatusCode < 500 {
		return Action{Kind: ActionDeadLetter}
	}
	remaining := p.MaxAttempts - attempt.Attempt.Count
	if remaining > 0 {
		bf := p.backoff(attempt.Attempt.Count)
		return Action{Kind: ActionRetry, BackoffDuration: bf}
	}
	return Action{Kind: ActionDeadLetter}
}

func (p DefaultDeliveryPolicy) backoff(attempt int) time.Duration {
	base := p.BaseBackoff
	if base == 0 {
		base = 1 * time.Second
	}
	d := base * time.Duration(math.Pow(2, float64(attempt-1)))
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}
