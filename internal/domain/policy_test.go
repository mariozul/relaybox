package domain_test

import (
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestDeliveryPolicy2xxDelivers(t *testing.T) {
	t.Parallel()
	p := domain.DefaultDeliveryPolicy{MaxAttempts: 5}
	action := p.Classify(&domain.DeliveryAttempt{
		StatusCode: 200,
		Attempt:    domain.AttemptInfo{Count: 1},
		Now:        time.Now().UTC(),
	})
	if action.Kind != domain.ActionDelivered {
		t.Fatalf("2xx must deliver, got %s", action.Kind)
	}
}

func TestDeliveryPolicy4xxDeadLetters(t *testing.T) {
	t.Parallel()
	p := domain.DefaultDeliveryPolicy{MaxAttempts: 5}
	action := p.Classify(&domain.DeliveryAttempt{
		StatusCode: 400,
		Attempt:    domain.AttemptInfo{Count: 1},
		Now:        time.Now().UTC(),
	})
	if action.Kind != domain.ActionDeadLetter {
		t.Fatalf("4xx must dead-letter immediately, got %s", action.Kind)
	}
}

func TestDeliveryPolicy5xxRetries(t *testing.T) {
	t.Parallel()
	p := domain.DefaultDeliveryPolicy{MaxAttempts: 5}
	action := p.Classify(&domain.DeliveryAttempt{
		StatusCode: 500,
		Attempt:    domain.AttemptInfo{Count: 1},
		Now:        time.Now().UTC(),
	})
	if action.Kind != domain.ActionRetry {
		t.Fatalf("5xx must retry, got %s", action.Kind)
	}
}

func TestDeliveryPolicyNetworkErrorRetries(t *testing.T) {
	t.Parallel()
	p := domain.DefaultDeliveryPolicy{MaxAttempts: 5}
	now := time.Now().UTC()
	action := p.Classify(&domain.DeliveryAttempt{
		StatusCode: 0,
		Attempt:    domain.AttemptInfo{Count: 1},
		Now:        now,
	})
	if action.Kind != domain.ActionRetry {
		t.Fatalf("network error (status 0) must retry, got %s", action.Kind)
	}
}

func TestDeliveryPolicyExhaustedDeadLetters(t *testing.T) {
	t.Parallel()
	p := domain.DefaultDeliveryPolicy{MaxAttempts: 3}
	action := p.Classify(&domain.DeliveryAttempt{
		StatusCode: 500,
		Attempt:    domain.AttemptInfo{Count: 3},
		Now:        time.Now().UTC(),
	})
	if action.Kind != domain.ActionDeadLetter {
		t.Fatalf("exhausted attempts must dead-letter, got %s", action.Kind)
	}
}

func TestDeliveryPolicyBackoffIncreases(t *testing.T) {
	t.Parallel()
	p := domain.DefaultDeliveryPolicy{MaxAttempts: 5, BaseBackoff: 1 * time.Second}
	a1 := p.Classify(&domain.DeliveryAttempt{
		StatusCode: 500,
		Attempt:    domain.AttemptInfo{Count: 1},
		Now:        time.Now().UTC(),
	})
	a2 := p.Classify(&domain.DeliveryAttempt{
		StatusCode: 500,
		Attempt:    domain.AttemptInfo{Count: 2},
		Now:        time.Now().UTC(),
	})
	if a1.BackoffDuration >= a2.BackoffDuration {
		t.Fatalf("backoff must increase: attempt 1 backoff=%v, attempt 2 backoff=%v", a1.BackoffDuration, a2.BackoffDuration)
	}
}
