package mystore

import (
	"context"
	"os"
)

type ctxTransactionKey struct{}

type Filter struct {
	Field   string
	Compare string
	Value   any
}

//go:generate mockgen -source=api.go -package mystore -destination store_mock.go Store
type Store[T any] interface {
	RunInTransaction(c context.Context, f func(c context.Context) error) error
	Put(c context.Context, uid string, value T) error
	Get(c context.Context, uid string) (T, bool, error)
	List(c context.Context) ([]T, error)
	Query(c context.Context, filters []Filter, orderByField string) ([]T, error)
}

func New[T any](c context.Context) (Store[T], func(), error) {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		return newGcloudStore[T](c)
	}

	return NewInMemoryStore[T](c)
}
