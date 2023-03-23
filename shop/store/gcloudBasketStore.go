package store

import (
	"context"

	"github.com/MarcGrol/shopbackend/mystore"
)

type gcloudPaymentStore struct {
	gcloudDatastoreClient mystore.DataStorer
}

func NewGcloudBasketStore(c context.Context) (BasketStore, error) {
	store, _, err := mystore.NewStore(c)
	if err != nil {
		return nil, err
	}
	return &gcloudPaymentStore{
		gcloudDatastoreClient: store,
	}, nil
}

func (s *gcloudPaymentStore) Put(ctx context.Context, basketUID string, paymentData Basket) error {
	return s.gcloudDatastoreClient.Put(ctx, "Basket", basketUID, &paymentData)
}

func (s *gcloudPaymentStore) Get(ctx context.Context, basketUID string) (Basket, bool, error) {
	basket := Basket{}
	exists, err := s.gcloudDatastoreClient.Get(ctx, "Basket", basketUID, &basket)
	if err != nil {
		return basket, false, err
	}
	return basket, exists, nil
}

func (s *gcloudPaymentStore) List(ctx context.Context) ([]Basket, error) {
	baskets := []Basket{}
	err := s.gcloudDatastoreClient.List(ctx, "Basket", baskets)
	if err != nil {
		return baskets, err
	}
	return baskets, nil
}
