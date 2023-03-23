package shop

import (
	"context"
	"sync"
)

type BasketStore interface {
	Put(ctx context.Context, basketUID string, basket Basket) error
	Get(ctx context.Context, basketUID string) (Basket, bool, error)
	List() ([]Basket, error)
}

type inMemoryPaymentStore struct {
	sync.Mutex
	baskets map[string]Basket
}

func NewBasketStore() BasketStore {
	return &inMemoryPaymentStore{
		baskets: map[string]Basket{},
	}
}
func (ps *inMemoryPaymentStore) Put(ctx context.Context, basketUID string, paymentData Basket) error {
	ps.Lock()
	defer ps.Unlock()

	ps.baskets[basketUID] = paymentData

	//log.Printf("Stored basket with uid %s", basketUID)

	return nil
}

func (ps *inMemoryPaymentStore) Get(ctx context.Context, basketUID string) (Basket, bool, error) {
	ps.Lock()
	defer ps.Unlock()

	basket, found := ps.baskets[basketUID]

	return basket, found, nil
}

func (ps *inMemoryPaymentStore) List() ([]Basket, error) {
	ps.Lock()
	defer ps.Unlock()

	baskets := []Basket{}
	for _, b := range ps.baskets {
		baskets = append(baskets, b)
	}
	return baskets, nil
}
