package store

import (
	"context"
	"os"
	"sync"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
)

type inMemoryCheckoutStore struct {
	sync.Mutex
	checkouts map[string]checkoutmodel.CheckoutContext
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		New = newInMemoryBasketStore
	}
}

func newInMemoryBasketStore(c context.Context) (CheckoutStorer, func(), error) {
	return &inMemoryCheckoutStore{
		checkouts: map[string]checkoutmodel.CheckoutContext{},
	}, func() {}, nil
}

func NewCheckoutStore() CheckoutStorer {
	return &inMemoryCheckoutStore{
		checkouts: map[string]checkoutmodel.CheckoutContext{},
	}
}
func (ps *inMemoryCheckoutStore) Put(ctx context.Context, checkoutUID string, checkout *checkoutmodel.CheckoutContext) error {
	ps.Lock()
	defer ps.Unlock()

	ps.checkouts[checkoutUID] = *checkout

	return nil
}

func (ps *inMemoryCheckoutStore) Get(ctx context.Context, checkoutUID string) (checkoutmodel.CheckoutContext, bool, error) {
	ps.Lock()
	defer ps.Unlock()

	checkout, found := ps.checkouts[checkoutUID]

	return checkout, found, nil
}
