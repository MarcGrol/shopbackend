package store

import (
	"context"
	"os"
	"sync"
)

type inMemoryCheckoutStore struct {
	sync.Mutex
	checkouts map[string]CheckoutContext
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		New = newInMemoryBasketStore
	}
}

func newInMemoryBasketStore(c context.Context) (CheckoutStorer, func(), error) {
	return &inMemoryCheckoutStore{
		checkouts: map[string]CheckoutContext{},
	}, func() {}, nil
}

func NewCheckoutStore() CheckoutStorer {
	return &inMemoryCheckoutStore{
		checkouts: map[string]CheckoutContext{},
	}
}
func (ps *inMemoryCheckoutStore) Put(ctx context.Context, checkoutUID string, checkout *CheckoutContext) error {
	ps.Lock()
	defer ps.Unlock()

	ps.checkouts[checkoutUID] = *checkout

	//log.Printf("Stored checkout with uid %s", checkoutUID)

	return nil
}

func (ps *inMemoryCheckoutStore) Get(ctx context.Context, checkoutUID string) (CheckoutContext, bool, error) {
	ps.Lock()
	defer ps.Unlock()

	checkout, found := ps.checkouts[checkoutUID]

	return checkout, found, nil
}
