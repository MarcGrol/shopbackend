package checkoutstripe

import (
	"context"
	"fmt"
	"net/url"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/services/checkoutadyen"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
)

type service struct {
	logger        mylog.Logger
	nower         mytime.Nower
	checkoutStore mystore.Store[checkoutadyen.CheckoutContext]
	publisher     mypublisher.Publisher
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(apiKey string, logger mylog.Logger, nower mytime.Nower, checkoutStore mystore.Store[checkoutadyen.CheckoutContext], publisher mypublisher.Publisher) (*service, error) {
	stripe.Key = apiKey
	return &service{
		logger:        logger,
		nower:         nower,
		checkoutStore: checkoutStore,
		publisher:     publisher,
	}, nil
}

// startCheckout starts a checkout session on the Adyen platform
func (s *service) startCheckout(c context.Context, basketUID string, returnURL string, params stripe.CheckoutSessionParams) (string, error) {
	now := s.nower.Now()

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

	session, err := session.New(&params)
	if err != nil {
		return "", myerrors.NewInvalidInputError(fmt.Errorf("error creating session: %s", err))
	}

	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		// Store checkout context because we need it later again
		err = s.checkoutStore.Put(c, basketUID, checkoutadyen.CheckoutContext{
			BasketUID:         basketUID,
			CreatedAt:         now,
			OriginalReturnURL: returnURL,
			ID:                session.ID,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err))
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName:  "stripe",
			CheckoutUID:   basketUID,
			AmountInCents: session.AmountTotal,
			Currency:      string(session.Currency),
			ShopperUID:    *params.ClientReferenceID,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return session.URL, nil
}

func (s *service) finalizeCheckout(c context.Context, basketUID string, status string) (string, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Redirect (start): Checkout completed for basket %s -> %s", basketUID, status)

	now := s.nower.Now()

	adjustedReturnURL := ""
	err := s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error fetching checkout with uid %s: %s", basketUID, err))
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID))
		}

		checkoutContext.Status = status
		checkoutContext.LastModified = &now

		err = s.checkoutStore.Put(c, basketUID, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		adjustedReturnURL, err = addStatusQueryParam(checkoutContext.OriginalReturnURL, status)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error adjusting url: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Redirect (done): Checkout completed for basket %s -> %s", basketUID, status)

	return adjustedReturnURL, nil
}

func addStatusQueryParam(orgUrl string, status string) (string, error) {
	u, err := url.Parse(orgUrl)
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error parsing return ReturnURL %s: %s", orgUrl, err))
	}
	params := u.Query()
	params.Set("status", status)
	u.RawQuery = params.Encode()
	return u.String(), nil
}
