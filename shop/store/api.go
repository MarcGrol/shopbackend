package store

import (
	"context"

	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

var New func(c context.Context) (BasketStorer, func(), error)

type BasketStorer interface {
	Put(ctx context.Context, basketUID string, basket *shopmodel.Basket) error
	Get(ctx context.Context, basketUID string) (shopmodel.Basket, bool, error)
	List(ctx context.Context) ([]shopmodel.Basket, error)
}
