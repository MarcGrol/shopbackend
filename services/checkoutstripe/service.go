package checkoutstripe

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
)

type service struct {
	logger mylog.Logger
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(logger mylog.Logger) (*service, error) {
	return &service{
		logger: logger,
	}, nil
}

// startCheckout starts a checkout session on the Stripe platform
func (s *service) startCheckout(c context.Context, basketUID string, req checkout.CreateCheckoutSessionRequest, returnURL string) (*CheckoutPageInfo, error) {
	return nil, myerrors.NewNotImplementedError(fmt.Errorf("not implemented yet"))
}
