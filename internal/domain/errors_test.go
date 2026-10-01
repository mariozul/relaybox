package domain_test

import (
	"errors"
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	t.Run("ErrNotFound", func(t *testing.T) {
		t.Parallel()
		if domain.ErrNotFound == nil {
			t.Fatal("ErrNotFound must not be nil")
		}
		if !errors.Is(domain.ErrNotFound, domain.ErrNotFound) {
			t.Fatal("ErrNotFound must match itself via errors.Is")
		}
	})

	t.Run("ErrConflict", func(t *testing.T) {
		t.Parallel()
		if domain.ErrConflict == nil {
			t.Fatal("ErrConflict must not be nil")
		}
		if !errors.Is(domain.ErrConflict, domain.ErrConflict) {
			t.Fatal("ErrConflict must match itself via errors.Is")
		}
	})

	t.Run("ErrInvalidState", func(t *testing.T) {
		t.Parallel()
		if domain.ErrInvalidState == nil {
			t.Fatal("ErrInvalidState must not be nil")
		}
		if !errors.Is(domain.ErrInvalidState, domain.ErrInvalidState) {
			t.Fatal("ErrInvalidState must match itself via errors.Is")
		}
	})

	t.Run("ErrTenantMismatch", func(t *testing.T) {
		t.Parallel()
		if domain.ErrTenantMismatch == nil {
			t.Fatal("ErrTenantMismatch must not be nil")
		}
		if !errors.Is(domain.ErrTenantMismatch, domain.ErrTenantMismatch) {
			t.Fatal("ErrTenantMismatch must match itself via errors.Is")
		}
	})

	t.Run("ErrMaxRetriesExceeded", func(t *testing.T) {
		t.Parallel()
		if domain.ErrMaxRetriesExceeded == nil {
			t.Fatal("ErrMaxRetriesExceeded must not be nil")
		}
		if !errors.Is(domain.ErrMaxRetriesExceeded, domain.ErrMaxRetriesExceeded) {
			t.Fatal("ErrMaxRetriesExceeded must match itself via errors.Is")
		}
	})
}
