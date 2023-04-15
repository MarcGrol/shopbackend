package myvault

import (
	"context"

	"github.com/MarcGrol/shopbackend/lib/mystore"
)

type vault struct {
	store mystore.Store[Token]
}

func New(c context.Context) (VaultReadWriter, func(), error) {
	store, storeCleanup, err := mystore.New[Token](c)
	if err != nil {
		return nil, nil, err
	}
	return &vault{
		store: store,
	}, storeCleanup, nil
}

func (v vault) Put(c context.Context, uid string, value Token) error {
	return v.store.Put(c, uid, value)
}

func (v vault) Get(c context.Context, uid string) (Token, bool, error) {
	return v.store.Get(c, uid)
}
