package myvault

import (
	"context"
)

type VaultReader[T any] interface {
	Get(c context.Context, uid string) (T, bool, error)
}

//go:generate mockgen -source=api.go -package myvault -destination vault_read_writer_mock.go VaultReadWriter
type VaultReadWriter[T any] interface {
	VaultReader[T]
	Put(c context.Context, uid string, value T) error
}
