package store

import (
	"context"
	"os"

	"github.com/MarcGrol/shopbackend/mystore"
)

type gcloudPaymentStore struct {
	gcloudDatastoreClient mystore.DataStorer
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		New = newGcloudBasketStore
	}
}

func newGcloudBasketStore(c context.Context) (BasketStorer, func(), error) {
	store, cleanup, err := mystore.NewStore(c)
	if err != nil {
		return nil, func() {}, err
	}
	return &gcloudPaymentStore{
		gcloudDatastoreClient: store,
	}, cleanup, nil
}

func (s *gcloudPaymentStore) Put(ctx context.Context, basketUID string, basket *Basket) error {
	return s.gcloudDatastoreClient.Put(ctx, "Basket", basketUID, basket)
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
	err := s.gcloudDatastoreClient.List(ctx, "Basket", &baskets)
	if err != nil {
		return baskets, err
	}
	return baskets, nil
}
