package store

import (
	"context"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/mystore"
)

type checkoutStore struct {
	store mystore.DataStorer
}

func New(c context.Context) (CheckoutStorer, func(), error) {
	ds, cleanup, err := mystore.New(c, func() (interface{}, interface{}) {
		return &checkoutmodel.CheckoutContext{}, &[]checkoutmodel.CheckoutContext{}
	})
	if err != nil {
		return nil, func() {}, err
	}
	return &checkoutStore{
		store: ds,
	}, cleanup, nil
}

func (s *checkoutStore) RunInTransaction(c context.Context, f func(c context.Context) error) error {
	return s.store.RunInTransaction(c, f)
}

func (cs *checkoutStore) Put(ctx context.Context, uid string, checkout *checkoutmodel.CheckoutContext) error {
	return cs.store.Put(ctx, "CheckoutContext", uid, checkout)
}

func (cs *checkoutStore) Get(ctx context.Context, uid string) (*checkoutmodel.CheckoutContext, bool, error) {
	result, found, err := cs.store.Get(ctx, "CheckoutContext", uid)
	if err != nil {
		return nil, false, err
	}
	return result.(*checkoutmodel.CheckoutContext), found, nil
}
