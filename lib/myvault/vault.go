package myvault

import (
	"context"

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

func New(c context.Context) (Vault, func(), error) {
	return mystore.New[Token](c)
}
