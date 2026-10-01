package domain

import (
	"testing"
	"time"
)

func TestDeliveryPolicy(t *testing.T) {
	t.Parallel()
	p := DefaultDeliveryPolicy()
	if p.BaseDelay != 1*time.Second || p.MaxDelay != 60*time.Second || p.MaxRetries != 8 {
		t.Fatal("default policy mismatch")
	}
	wants := []time.Duration{1, 2, 4, 8, 16, 32, 60, 60}
	for i, w := range wants {
		if g := p.NextBackoff(i + 1); g != w*time.Second {
			t.Errorf("attempt %d: got %v, want %v", i+1, g, w*time.Second)
		}
	}
	if g := p.NextBackoff(0); g != 1*time.Second {
		t.Errorf("NextBackoff(0)=%v", g)
	}
	if p.IsExhausted(7) || !p.IsExhausted(8) {
		t.Error("IsExhausted boundary wrong")
	}
	cp := DeliveryPolicy{BaseDelay: 500 * time.Millisecond, MaxDelay: 5 * time.Second, MaxRetries: 3}
	if g := cp.NextBackoff(3); g != 2*time.Second {
		t.Errorf("custom backoff(3)=%v", g)
	}
}
