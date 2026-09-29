package service

import (
	"context"

	"github.com/mariozul/relaybox/pkg/domain"
)

type Subscriptions struct{ Store domain.SubscriptionStore }

func (s Subscriptions) Create(ctx context.Context, i domain.Identity, input domain.SubscriptionInput) (domain.Subscription, error) {
	if err := i.Validate(); err != nil {
		return domain.Subscription{}, err
	}
	return s.Store.CreateSubscription(ctx, i, input)
}
func (s Subscriptions) List(ctx context.Context, i domain.Identity) ([]domain.Subscription, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	return s.Store.ListSubscriptions(ctx, i)
}
func (s Subscriptions) Delete(ctx context.Context, i domain.Identity, id string) error {
	if err := i.Validate(); err != nil {
		return err
	}
	return s.Store.DeleteSubscription(ctx, i, id)
}
