package store

import (
	"context"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
)

type CheckoutStorer interface {
	RunInTransaction(c context.Context, f func(c context.Context) error) error
	Put(ctx context.Context, checkoutUID string, checkout *checkoutmodel.CheckoutContext) error
	Get(ctx context.Context, checkoutUID string) (*checkoutmodel.CheckoutContext, bool, error)
}
