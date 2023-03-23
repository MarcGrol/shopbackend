package store

import (
	"context"
)

type BasketStore interface {
	Put(ctx context.Context, basketUID string, basket Basket) error
	Get(ctx context.Context, basketUID string) (Basket, bool, error)
	List(ctx context.Context) ([]Basket, error)
}
