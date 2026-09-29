package service

import (
	"context"

	"github.com/mariozul/relaybox/pkg/domain"
)

type Events struct{ Store domain.EventStore }

func (s Events) Ingest(ctx context.Context, i domain.Identity, input domain.EventInput) (domain.Event, error) {
	if err := i.Validate(); err != nil {
		return domain.Event{}, err
	}
	return s.Store.Ingest(ctx, i, input)
}
