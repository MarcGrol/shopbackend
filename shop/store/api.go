package store

import (
	"context"
)

var New func(c context.Context) (BasketStorer, func(), error)

type BasketStorer interface {
	Put(ctx context.Context, basketUID string, basket *Basket) error
	Get(ctx context.Context, basketUID string) (Basket, bool, error)
	List(ctx context.Context) ([]Basket, error)
}
