package domain

import (
	"errors"
	"testing"
	"time"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func TestOutboxEntry_MarkDelivered(t *testing.T) {
	t.Parallel()

	// pending → delivered
	o := &OutboxEntry{Status: OutboxStatusPending}
	if err := o.MarkDelivered(); err != nil || o.Status != OutboxStatusDelivered {
		t.Fatalf("pending→delivered: err=%v status=%v", err, o.Status)
	}
	// idempotent: delivered → delivered
	if err := o.MarkDelivered(); err != nil {
		t.Fatalf("idempotent delivered: %v", err)
	}

	// failed → delivered
	f := &OutboxEntry{Status: OutboxStatusFailed}
	if err := f.MarkDelivered(); err != nil || f.Status != OutboxStatusDelivered {
		t.Fatalf("failed→delivered: err=%v status=%v", err, f.Status)
	}

	// deadletter → delivered (should fail)
	d := &OutboxEntry{Status: OutboxStatusDeadLetter}
	if err := d.MarkDelivered(); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("deadletter→delivered: expected ErrInvalidState, got %v", err)
	}
}

func TestOutboxEntry_MarkFailed(t *testing.T) {
	t.Parallel()

	// pending → failed
	o := &OutboxEntry{Status: OutboxStatusPending}
	if err := o.MarkFailed(); err != nil || o.Status != OutboxStatusFailed {
		t.Fatalf("pending→failed: err=%v status=%v", err, o.Status)
	}

	// failed → failed (idempotent)
	if err := o.MarkFailed(); err != nil || o.Status != OutboxStatusFailed {
		t.Fatalf("failed→failed idempotent: err=%v status=%v", err, o.Status)
	}

	// terminal states → failed (should fail)
	for _, st := range []OutboxStatus{OutboxStatusDelivered, OutboxStatusDeadLetter} {
		d := &OutboxEntry{Status: st}
		if err := d.MarkFailed(); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("%s→failed: expected ErrInvalidState, got %v", st, err)
		}
	}
}

func TestOutboxEntry_MarkDeadLetter(t *testing.T) {
	t.Parallel()

	// pending → deadletter
	o := &OutboxEntry{Status: OutboxStatusPending}
	if err := o.MarkDeadLetter(); err != nil || o.Status != OutboxStatusDeadLetter {
		t.Fatalf("pending→deadletter: err=%v status=%v", err, o.Status)
	}
	// idempotent: deadletter → deadletter
	if err := o.MarkDeadLetter(); err != nil {
		t.Fatalf("idempotent deadletter: %v", err)
	}

	// failed → deadletter
	f := &OutboxEntry{Status: OutboxStatusFailed}
	if err := f.MarkDeadLetter(); err != nil || f.Status != OutboxStatusDeadLetter {
		t.Fatalf("failed→deadletter: err=%v status=%v", err, f.Status)
	}

	// delivered → deadletter (should fail)
	d := &OutboxEntry{Status: OutboxStatusDelivered}
	if err := d.MarkDeadLetter(); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("delivered→deadletter: expected ErrInvalidState, got %v", err)
	}
}

func TestOutboxEntry_IncrementAttempts(t *testing.T) {
	t.Parallel()

	c := fixedClock{t: time.Now()}

	// pending: increment once
	o := &OutboxEntry{Status: OutboxStatusPending}
	if err := o.IncrementAttempts(c); err != nil || o.Attempts != 1 {
		t.Fatalf("inc1: err=%v attempts=%d", err, o.Attempts)
	}
	if err := o.IncrementAttempts(c); err != nil || o.Attempts != 2 {
		t.Fatalf("inc2: err=%v attempts=%d", err, o.Attempts)
	}

	// failed: increment works
	f := &OutboxEntry{Status: OutboxStatusFailed}
	if err := f.IncrementAttempts(c); err != nil || f.Attempts != 1 {
		t.Fatalf("failed inc1: err=%v attempts=%d", err, f.Attempts)
	}

	// terminal states: no increment allowed
	for _, st := range []OutboxStatus{OutboxStatusDelivered, OutboxStatusDeadLetter} {
		od := &OutboxEntry{Status: st}
		if err := od.IncrementAttempts(c); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("increment on %v: expected ErrInvalidState, got %v", st, err)
		}
	}
}

func TestOutboxEntry_IsTerminal(t *testing.T) {
	t.Parallel()

	if (&OutboxEntry{Status: OutboxStatusPending}).IsTerminal() {
		t.Error("pending should not be terminal")
	}
	if (&OutboxEntry{Status: OutboxStatusFailed}).IsTerminal() {
		t.Error("failed should not be terminal")
	}
	if !(&OutboxEntry{Status: OutboxStatusDelivered}).IsTerminal() {
		t.Error("delivered should be terminal")
	}
	if !(&OutboxEntry{Status: OutboxStatusDeadLetter}).IsTerminal() {
		t.Error("deadletter should be terminal")
	}
}
