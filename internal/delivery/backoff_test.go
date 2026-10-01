package delivery

import (
	"testing"
	"time"
)

func TestBackoff_ExponentialWithJitter(t *testing.T) {
	t.Parallel()

	// First attempt (attempts=0).
	delay, ok := Backoff(0)
	if !ok {
		t.Fatal("should allow retry on attempt 0")
	}
	if delay < 800*time.Millisecond || delay > 1200*time.Millisecond {
		t.Fatalf("expected ~1s base delay, got %v", delay)
	}

	// Second attempt (attempts=1).
	delay2, ok2 := Backoff(1)
	if !ok2 {
		t.Fatal("should allow retry on attempt 1")
	}
	if delay2 < 1600*time.Millisecond || delay2 > 2400*time.Millisecond {
		t.Fatalf("expected ~2s delay, got %v", delay2)
	}

	// Max attempts exceeded.
	_, ok3 := Backoff(5)
	if ok3 {
		t.Fatal("should NOT allow retry at max attempts")
	}
}

func TestBackoff_MaxAttempts(t *testing.T) {
	t.Parallel()
	if MaxAttempts() != 5 {
		t.Fatalf("expected max attempts=5, got %d", MaxAttempts())
	}
}
