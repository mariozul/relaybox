package domain

import (
	"testing"
	"time"
)

func TestRetryPolicy(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	policy := RetryPolicy{PermanentAttempts: 5, Base: time.Second, Cap: 5 * time.Minute}
	for _, tc := range []struct {
		name             string
		status, attempts int
		state            State
		delay            time.Duration
	}{
		{"success", 200, 1, Delivered, 0},
		{"last success", 299, 8, Delivered, 0},
		{"transient", 500, 1, Pending, time.Second},
		{"backoff", 503, 3, Pending, 4 * time.Second},
		{"timeout", 0, 1, Pending, time.Second},
		{"permanent retry", 400, 4, Pending, 8 * time.Second},
		{"permanent end", 400, 5, Deadletter, 0},
		{"429", 429, 5, Deadletter, 0},
		{"redirect", 302, 5, Pending, 16 * time.Second},
		{"cap", 500, 100000, Pending, 5 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.Decide(now, tc.status, tc.attempts)
			if got.State != tc.state {
				t.Fatalf("state %s", got.State)
			}
			if tc.state == Pending && !got.NextAttemptAt.Equal(now.Add(tc.delay)) {
				t.Fatalf("delay %v", got.NextAttemptAt.Sub(now))
			}
		})
	}
}

func TestRetryDefaults(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	got := (RetryPolicy{}).Decide(now, 500, 0)
	if got.State != Pending || !got.NextAttemptAt.Equal(now.Add(time.Second)) {
		t.Fatal(got)
	}
	got = (RetryPolicy{Base: time.Hour, Cap: time.Minute}).Decide(now, 500, 1)
	if !got.NextAttemptAt.Equal(now.Add(time.Minute)) {
		t.Fatal(got)
	}
}
