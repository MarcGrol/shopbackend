package store

import (
	"context"

	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

type BasketStorer interface {
	RunInTransaction(c context.Context, f func(context.Context) error) error
	Put(ctx context.Context, basketUID string, basket *shopmodel.Basket) error
	Get(ctx context.Context, basketUID string) (*shopmodel.Basket, bool, error)
	List(ctx context.Context) ([]shopmodel.Basket, error)
}
