package store

import "context"

var New func(c context.Context) (CheckoutStorer, func(), error)

type CheckoutStorer interface {
	Put(ctx context.Context, checkoutUID string, checkout *CheckoutContext) error
	Get(ctx context.Context, checkoutUID string) (CheckoutContext, bool, error)
}
