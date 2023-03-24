package store

import (
	"context"
	"os"
	"sync"

	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

type inMemoryPaymentStore struct {
	sync.Mutex
	baskets map[string]shopmodel.Basket
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		New = newInMemoryBasketStore
	}
}

func newInMemoryBasketStore(c context.Context) (BasketStorer, func(), error) {
	return &inMemoryPaymentStore{
		baskets: map[string]shopmodel.Basket{},
	}, func() {}, nil
}

func (ps *inMemoryPaymentStore) Put(ctx context.Context, basketUID string, basket *shopmodel.Basket) error {
	ps.Lock()
	defer ps.Unlock()

	ps.baskets[basketUID] = *basket

	//log.Printf("Stored basket with uid %s", basketUID)

	return nil
}

func (ps *inMemoryPaymentStore) Get(ctx context.Context, basketUID string) (shopmodel.Basket, bool, error) {
	ps.Lock()
	defer ps.Unlock()

	basket, found := ps.baskets[basketUID]

	return basket, found, nil
}

func (ps *inMemoryPaymentStore) List(c context.Context) ([]shopmodel.Basket, error) {
	ps.Lock()
	defer ps.Unlock()

	baskets := []shopmodel.Basket{}
	for _, b := range ps.baskets {
		baskets = append(baskets, b)
	}
	return baskets, nil
}
