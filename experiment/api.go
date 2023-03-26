package experiment

import (
	"context"
	"os"
)

type Store[T any] interface {
	RunInTransaction(c context.Context, f func(c context.Context) error) error
	Put(c context.Context, uid string, value T) error
	Get(c context.Context, uid string) (T, bool, error)
	List(c context.Context) ([]T, error)
}

func New[T any](c context.Context) (Store[T], func(), error) {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		return newGcloudStore[T](c)
	}
	return newInMemoryStore[T](c)
}
