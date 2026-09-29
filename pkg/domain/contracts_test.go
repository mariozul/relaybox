package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestIdentityValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		identity Identity
		valid    bool
	}{
		{"valid", Identity{"tenant", "actor"}, true},
		{"missing tenant", Identity{"", "actor"}, false},
		{"missing actor", Identity{"tenant", ""}, false},
		{"padded", Identity{" tenant", "actor"}, false},
		{"control", Identity{"tenant", "a\nb"}, false},
		{"long", Identity{strings.Repeat("a", 257), "actor"}, false},
		{"boundary", Identity{strings.Repeat("a", 256), "actor"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.identity.Validate()
			if tc.valid && err != nil {
				t.Fatal(err)
			}
			if !tc.valid && !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
