package myvault

import (
	"context"
	"time"
)

const (
	CurrentToken = "currentToken"
)

type Token struct {
	ProviderName string
	ClientID     string
	SessionUID   string
	Scopes       string
	CreatedAt    time.Time
	LastModified *time.Time
	AccessToken  string
	RefreshToken string
	ExpiresIn    *time.Time
}

type VaultReader interface {
	Get(c context.Context, uid string) (Token, bool, error)
}

//go:generate mockgen -source=api.go -package myvault -destination vault_read_writer_mock.go VaultReadWriter
type VaultReadWriter interface {
	Get(c context.Context, uid string) (Token, bool, error)
	Put(c context.Context, uid string, value Token) error
}
