package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestTenantIsRequired(t *testing.T) {
	t.Parallel()
	s := &Store{}
	_, _, err := s.Ingest(context.Background(), domain.Event{EventType: "created", DedupKey: "key", Payload: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("expected missing tenant error")
	}
}
