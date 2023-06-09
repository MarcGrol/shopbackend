package checkoutmollie

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/VictorAvelar/mollie-api-go/v3/mollie"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
)

type service struct {
	apiKey        string
	payer         Payer
	logger        mylog.Logger
	nower         mytime.Nower
	checkoutStore mystore.Store[checkoutapi.CheckoutContext]
	vault         myvault.VaultReader
	publisher     mypublisher.Publisher
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(apiKey string, payer Payer, logger mylog.Logger, nower mytime.Nower, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader, publisher mypublisher.Publisher) (*service, error) {
	return &service{
		apiKey:        apiKey,
		payer:         payer,
		logger:        logger,
		nower:         nower,
		checkoutStore: checkoutStore,
		vault:         vault,
		publisher:     publisher,
	}, nil
}

func (s *service) startCheckout(c context.Context, basketUID string, returnURL string, request mollie.Payment) (string, error) {
	now := s.nower.Now()

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

	// Iniitialize payment to the stripe platform
	s.setupAuthentication(c, basketUID)
	paymentResp, err := s.payer.CreatePayment(c, request)
	if err != nil {
		return "", myerrors.NewInvalidInputError(err)
	}

	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		// Store checkout context on basketUID because we need it for the success/cancel callback and the webhook
		err = s.checkoutStore.Put(c, basketUID, checkoutapi.CheckoutContext{
			BasketUID:         basketUID,
			CreatedAt:         now,
			OriginalReturnURL: returnURL,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err))
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName: "mollie",
			CheckoutUID:  basketUID,
			AmountInCents: func() int64 {
				value, _ := strconv.ParseFloat(request.Amount.Value, 64)
				return int64(value * 100)
			}(),
			Currency:   request.Amount.Currency,
			ShopperUID: request.CustomerReference,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s completed", basketUID)

	return paymentResp.Links.Checkout.Href, nil
}

func (s *service) setupAuthentication(c context.Context, basketUID string) {
	tokenUID := myvault.CurrentToken + "_" + ("mollie")
	accessToken, exist, err := s.vault.Get(c, tokenUID)
	if err != nil || !exist || accessToken.ProviderName != "mollie" || accessToken.SessionUID == "" {
		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using api key")
		s.payer.UseAPIKey(s.apiKey)
	} else {
		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using access token")
		s.payer.UseToken(accessToken.AccessToken)
	}
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

func addStatusQueryParam(orgURL string, status string) (string, error) {
	u, err := url.Parse(orgURL)
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error parsing return ReturnURL %s: %s", orgURL, err))
	}
	params := u.Query()
	params.Set("status", status)
	u.RawQuery = params.Encode()
	return u.String(), nil
}

func (s *service) webhookNotification(c context.Context, username, password string, basketUID string, id string) error {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Webhook: status update event '%s'", id)

	payment, err := s.payer.GetPaymentOnID(c, id)
	if err != nil {
		return myerrors.NewInternalError(fmt.Errorf("error getting payment %s on id: %s", id, err))
	}

	fmt.Printf("Payment: %v", payment)

	now := s.nower.Now()

	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID))
		}

		checkoutContext.PaymentMethod = string(payment.Method)
		eventStatus := classifyEventStatus(payment.Status)
		checkoutContext.LastModified = &now
		checkoutContext.CheckoutStatus = eventStatus
		checkoutContext.CheckoutStatusDetails = payment.Status

		err = s.checkoutStore.Put(c, checkoutContext.BasketUID, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
			ProviderName:          "mollie",
			CheckoutUID:           checkoutContext.BasketUID,
			PaymentMethod:         checkoutContext.PaymentMethod,
			CheckoutStatus:        eventStatus,
			CheckoutStatusDetails: payment.Status,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func classifyEventStatus(mollieStatus string) checkoutevents.CheckoutStatus {
	switch mollieStatus {
	case "paid":
		return checkoutevents.CheckoutStatusSuccess
	case "canceled":
		return checkoutevents.CheckoutStatusCancelled
	case "failed":
		return checkoutevents.CheckoutStatusFailed
	case "expired":
		return checkoutevents.CheckoutStatusExpired

	default:
		return checkoutevents.CheckoutStatusOther
	}
}
