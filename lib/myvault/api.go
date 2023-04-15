package myvault

import (
	"context"
)

const (
	CurrentToken = "currentToken"
)

type Token struct {
	ClientID     string
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

type VaultReader interface {
	Get(c context.Context, uid string) (Token, bool, error)
}

//go:generate mockgen -source=api.go -package myvault -destination vault_read_writer_mock.go VaultReadWriter
type VaultReadWriter interface {
	Get(c context.Context, uid string) (Token, bool, error)
	Put(c context.Context, uid string, value Token) error
}
