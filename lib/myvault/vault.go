package myvault

import (
	"context"
	"encoding/json"

	"github.com/MarcGrol/shopbackend/lib/mystore"
)

const (
	CurrentToken = "currentToken"
)

type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

//go:generate mockgen -source=vault.go -package myvault -destination vault_mock.go Vault
type Vault interface {
	Put(c context.Context, uid string, value Token) error
	Get(c context.Context, uid string) (Token, bool, error)
}

type vault struct {
	store mystore.Store[[]byte]
}

func New(c context.Context) (Vault, func(), error) {
	store, storeCleanup, err := mystore.New[[]byte](c)
	if err != nil {
		return nil, nil, err
	}
	return &vault{
		store: store,
	}, storeCleanup, nil
}

func (v vault) Put(c context.Context, uid string, value Token) error {
	jsonBytes, err := json.MarshalIndent(value, "", "\t")
	if err != nil {
		return err
	}
	return v.store.Put(c, uid, jsonBytes)
}

func (v vault) Get(c context.Context, uid string) (Token, bool, error) {
	token := Token{}

	jsonBytes, exists, err := v.store.Get(c, uid)
	if err != nil {
		return token, false, err
	}
	if !exists {
		return token, false, nil
	}
	err = json.Unmarshal(jsonBytes, &token)
	if err != nil {
		return token, true, err
	}
	return token, true, nil
}
