package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sentinel error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrConflict", ErrConflict},
		{"ErrInvalidInput", ErrInvalidInput},
		{"ErrDuplicateEvent", ErrDuplicateEvent},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.sentinel == nil {
				t.Fatal("sentinel must not be nil")
			}
			if tt.sentinel.Error() == "" {
				t.Fatal("sentinel must have a non-empty error message")
			}
		})
	}
}

func TestErrorWrappingAndIs(t *testing.T) {
	t.Parallel()

	t.Run("ErrNotFound wrapping", func(t *testing.T) {
		err := fmt.Errorf("get event: %w", ErrNotFound)
		if !IsNotFound(err) {
			t.Fatal("IsNotFound should detect wrapped ErrNotFound")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Fatal("errors.Is should detect wrapped ErrNotFound")
		}
	})

	t.Run("ErrConflict wrapping", func(t *testing.T) {
		err := fmt.Errorf("create subscription: %w", ErrConflict)
		if !IsConflict(err) {
			t.Fatal("IsConflict should detect wrapped ErrConflict")
		}
	})

	t.Run("ErrInvalidInput wrapping", func(t *testing.T) {
		err := fmt.Errorf("validate: %w", ErrInvalidInput)
		if !IsInvalidInput(err) {
			t.Fatal("IsInvalidInput should detect wrapped ErrInvalidInput")
		}
	})

	t.Run("non-sentinel error is not NotFound", func(t *testing.T) {
		if IsNotFound(errors.New("some random error")) {
			t.Fatal("IsNotFound should not match unrelated error")
		}
	})
}
