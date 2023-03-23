package store

import (
	"context"
	"sync"
)

type CheckoutStore interface {
	Put(ctx context.Context, checkoutUID string, checkout CheckoutContext) error
	Get(ctx context.Context, checkoutUID string) (CheckoutContext, bool, error)
}

type inMemoryCheckoutStore struct {
	sync.Mutex
	checkouts map[string]CheckoutContext
}

func NewCheckoutStore() CheckoutStore {
	return &inMemoryCheckoutStore{
		checkouts: map[string]CheckoutContext{},
	}
}
func (ps *inMemoryCheckoutStore) Put(ctx context.Context, checkoutUID string, checkout CheckoutContext) error {
	ps.Lock()
	defer ps.Unlock()

	ps.checkouts[checkoutUID] = checkout

	//log.Printf("Stored checkout with uid %s", checkoutUID)

	return nil
}

func (ps *inMemoryCheckoutStore) Get(ctx context.Context, checkoutUID string) (CheckoutContext, bool, error) {
	ps.Lock()
	defer ps.Unlock()

	checkout, found := ps.checkouts[checkoutUID]

	return checkout, found, nil
}
