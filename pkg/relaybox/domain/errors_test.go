package domain_test

import (
	"errors"
	"testing"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

func TestDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantMsg  string
		wantCode string
	}{
		{
			name:     "ErrEventAlreadyExists",
			err:      domain.ErrEventAlreadyExists,
			wantMsg:  "event already exists for this tenant with the same dedup key",
			wantCode: domain.ErrCodeEventAlreadyExists,
		},
		{
			name:     "ErrSubscriptionNotFound",
			err:      domain.ErrSubscriptionNotFound,
			wantMsg:  "subscription not found or access denied",
			wantCode: domain.ErrCodeSubscriptionNotFound,
		},
		{
			name:     "ErrUnauthorized",
			err:      domain.ErrUnauthorized,
			wantMsg:  "unauthorized access to resource",
			wantCode: domain.ErrCodeUnauthorized,
		},
		{
			name:     "ErrInvalidInput",
			err:      domain.ErrInvalidInput,
			wantMsg:  "invalid input provided",
			wantCode: domain.ErrCodeInvalidInput,
		},
		{
			name:     "ErrInternal",
			err:      domain.ErrInternal,
			wantMsg:  "internal server error",
			wantCode: domain.ErrCodeInternal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Error() != tc.wantMsg {
				t.Errorf("expected message %q, got %q", tc.wantMsg, tc.err.Error())
			}
			// Verify errors.Is works for sentinels
			if !errors.Is(tc.err, tc.err) {
				t.Errorf("errors.Is should return true for itself")
			}
		})
	}
}

func TestErrorCodeConstants(t *testing.T) {
	t.Parallel()

	if domain.ErrCodeEventAlreadyExists != "EVENT_ALREADY_EXISTS" {
		t.Errorf("unexpected code: %s", domain.ErrCodeEventAlreadyExists)
	}
	if domain.ErrCodeSubscriptionNotFound != "SUBSCRIPTION_NOT_FOUND" {
		t.Errorf("unexpected code: %s", domain.ErrCodeSubscriptionNotFound)
	}
	if domain.ErrCodeUnauthorized != "UNAUTHORIZED_ACCESS" {
		t.Errorf("unexpected code: %s", domain.ErrCodeUnauthorized)
	}
	if domain.ErrCodeInvalidInput != "INVALID_INPUT" {
		t.Errorf("unexpected code: %s", domain.ErrCodeInvalidInput)
	}
	if domain.ErrCodeInternal != "INTERNAL_SERVER_ERROR" {
		t.Errorf("unexpected code: %s", domain.ErrCodeInternal)
	}
}
