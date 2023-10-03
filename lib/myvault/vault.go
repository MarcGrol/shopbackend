package myvault

import (
	"context"

	"github.com/MarcGrol/shopbackend/lib/mystore"
)

type vault[T any] struct {
	store mystore.Store[T]
}

func NewReaderWriter[T any](c context.Context) (VaultReadWriter[T], func(), error) {
	store, storeCleanup, err := mystore.New[T](c)
	if err != nil {
		return nil, nil, err
	}

	return &vault[T]{
		store: store,
	}, storeCleanup, nil
}

func NewReader[T any](c context.Context) (VaultReader[T], func(), error) {
	store, storeCleanup, err := mystore.New[T](c)
	if err != nil {
		return nil, nil, err
	}

	return &vault[T]{
		store: store,
	}, storeCleanup, nil
}

func (v vault[T]) Put(c context.Context, uid string, value T) error {
	return v.store.Put(c, uid, value)
}

func (v vault[T]) Get(c context.Context, uid string) (T, bool, error) {
	return v.store.Get(c, uid)
}
