package store

import (
	"context"
	"sort"

	"github.com/MarcGrol/shopbackend/mystore"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

type basketStore struct {
	store mystore.DataStorer
}

func New(c context.Context) (BasketStorer, func(), error) {
	ds, cleanup, err := mystore.New(c, func() (interface{}, interface{}) {
		return &shopmodel.Basket{}, &[]shopmodel.Basket{}
	})
	if err != nil {
		return nil, func() {}, err
	}
	return &basketStore{
		store: ds,
	}, cleanup, nil
}

func (s *basketStore) RunInTransaction(c context.Context, f func(c context.Context) error) error {
	return s.store.RunInTransaction(c, f)
}

func (bs *basketStore) Put(ctx context.Context, basketUID string, basket *shopmodel.Basket) error {
	return bs.store.Put(ctx, "Basket", basketUID, basket)
}

func (bs *basketStore) Get(ctx context.Context, basketUID string) (*shopmodel.Basket, bool, error) {
	result, found, err := bs.store.Get(ctx, "Basket", basketUID)
	if err != nil {
		return nil, false, err
	}
	return result.(*shopmodel.Basket), found, nil
}

func (bs *basketStore) List(ctx context.Context) ([]shopmodel.Basket, error) {
	result, err := bs.store.List(ctx, "Basket")
	if err != nil {
		return []shopmodel.Basket{}, err
	}

	// TODO Fix this ugly type conversion
	typedResult := []shopmodel.Basket{}
	_, ok := result.(*[]shopmodel.Basket)
	if ok {
		for _, i := range *result.(*[]shopmodel.Basket) {
			typedResult = append(typedResult, i)
		}
	}
	_, ok = result.([]interface{})
	if ok {
		for _, i := range result.([]interface{}) {
			typedResult = append(typedResult, *i.(*shopmodel.Basket))
		}
	}

	// Sort by creation date descending
	sort.Slice(typedResult, func(i, j int) bool {
		return typedResult[i].CreatedAt.After(typedResult[j].CreatedAt)
	})

	return typedResult, nil
}
