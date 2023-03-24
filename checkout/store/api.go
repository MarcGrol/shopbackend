package store

import (
	"context"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
)

var New func(c context.Context) (CheckoutStorer, func(), error)

type CheckoutStorer interface {
	Put(ctx context.Context, checkoutUID string, checkout *checkoutmodel.CheckoutContext) error
	Get(ctx context.Context, checkoutUID string) (checkoutmodel.CheckoutContext, bool, error)
}
