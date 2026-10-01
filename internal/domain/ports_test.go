package domain_test

import (
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestPortsExist(t *testing.T) {
	t.Parallel()

	t.Run("EventStore", func(t *testing.T) {
		t.Parallel()
		var _ domain.EventStore = nil
	})

	t.Run("SubscriptionStore", func(t *testing.T) {
		t.Parallel()
		var _ domain.SubscriptionStore = nil
	})

	t.Run("OutboxStore", func(t *testing.T) {
		t.Parallel()
		var _ domain.OutboxStore = nil
	})

	t.Run("TxManager", func(t *testing.T) {
		t.Parallel()
		var _ domain.TxManager = nil
	})
}
