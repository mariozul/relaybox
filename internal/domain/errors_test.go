package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	sentinels := map[string]error{
		"ErrNotFound":       ErrNotFound,
		"ErrConflict":       ErrConflict,
		"ErrInvalidInput":   ErrInvalidInput,
		"ErrDuplicateEvent": ErrDuplicateEvent,
		"ErrInvalidState":   ErrInvalidState,
	}

	for name, sentinel := range sentinels {
		name, sentinel := name, sentinel
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(sentinel, sentinel) || sentinel.Error() == "" {
				t.Errorf("%s is invalid", name)
			}
		})
	}

	// All sentinel errors must be distinct (none wraps another).
	all := []error{ErrNotFound, ErrConflict, ErrInvalidInput, ErrDuplicateEvent, ErrInvalidState}
	for i, a := range all {
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d wraps sentinel %d — they must be distinct", i, j)
			}
		}
	}
}

func TestErrInvalidInputWrapping(t *testing.T) {
	t.Parallel()

	_, err := NewEvent("t", "", json.RawMessage(`{}`), "dk", "sys")
	if err == nil {
		t.Fatal("expected error for empty event_type")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("NewEvent error should wrap ErrInvalidInput, got %v", err)
	}

	_, err = NewEvent("t", "e", nil, "dk", "sys")
	if err == nil {
		t.Fatal("expected error for nil payload")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("NewEvent error should wrap ErrInvalidInput, got %v", err)
	}
}
