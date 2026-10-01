package domain

import (
	"testing"
	"time"
)

func TestExponentialBackoff(t *testing.T) {
	t.Parallel()

	base := 1 * time.Second
	max := 1 * time.Hour
	factor := 2.0

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"attempt 0", 0, 0},
		{"attempt -1", -1, 0},
		{"attempt 1 → base", 1, 1 * time.Second},
		{"attempt 2 → base*2", 2, 2 * time.Second},
		{"attempt 3 → base*4", 3, 4 * time.Second},
		{"attempt 4 → base*8", 4, 8 * time.Second},
		{"attempt 5 → base*16", 5, 16 * time.Second},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExponentialBackoff(base, max, factor, tt.attempt)
			if got != tt.want {
				t.Errorf("ExponentialBackoff(base=%v, max=%v, factor=%v, attempt=%d) = %v, want %v",
					base, max, factor, tt.attempt, got, tt.want)
			}
		})
	}
}

func TestExponentialBackoffMaxCap(t *testing.T) {
	t.Parallel()

	base := 1 * time.Second
	max := 1 * time.Hour
	factor := 2.0

	for a := 1; a <= 30; a++ {
		got := ExponentialBackoff(base, max, factor, a)
		if got > max {
			t.Errorf("ExponentialBackoff(attempt=%d) = %v exceeds max %v", a, got, max)
		}
		if got < 0 {
			t.Errorf("ExponentialBackoff(attempt=%d) = %v negative", a, got)
		}
	}
}

func TestExponentialBackoffMonotonic(t *testing.T) {
	t.Parallel()

	base := 1 * time.Second
	max := 1 * time.Hour
	factor := 2.0

	prev := time.Duration(0)
	for a := 1; a <= 15; a++ {
		got := ExponentialBackoff(base, max, factor, a)
		if got < prev {
			t.Errorf("not monotonic: attempt %d gave %v, previous %v", a, got, prev)
		}
		prev = got
	}
}

func TestExponentialBackoffCustomFactor(t *testing.T) {
	t.Parallel()

	base := 100 * time.Millisecond
	max := 10 * time.Second
	factor := 3.0

	// 100ms * 3^0 = 100ms, 100ms * 3^1 = 300ms, 100ms * 3^2 = 900ms
	wants := []time.Duration{0, 100 * time.Millisecond, 300 * time.Millisecond, 900 * time.Millisecond, 2700 * time.Millisecond}
	for a, want := range wants {
		got := ExponentialBackoff(base, max, factor, a)
		if got != want {
			t.Errorf("ExponentialBackoff(base=%v, max=%v, factor=%v, attempt=%d) = %v, want %v",
				base, max, factor, a, got, want)
		}
	}
}

func TestExponentialBackoffSmallMax(t *testing.T) {
	t.Parallel()

	base := 5 * time.Second
	max := 1 * time.Second // max < base
	factor := 2.0

	// When base > max, attempt 1 should return max
	got := ExponentialBackoff(base, max, factor, 1)
	if got != max {
		t.Errorf("when base > max, attempt 1 should return max; got %v, want %v", got, max)
	}

	got = ExponentialBackoff(base, max, factor, 2)
	if got != max {
		t.Errorf("when base > max, attempt 2 should also be capped at max; got %v, want %v", got, max)
	}
}
