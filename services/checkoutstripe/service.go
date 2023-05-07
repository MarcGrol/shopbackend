package checkoutstripe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutadyen"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
)

type service struct {
	apiKey        string
	logger        mylog.Logger
	nower         mytime.Nower
	checkoutStore mystore.Store[checkoutadyen.CheckoutContext]
	vault         myvault.VaultReader
	publisher     mypublisher.Publisher
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(apiKey string, logger mylog.Logger, nower mytime.Nower, checkoutStore mystore.Store[checkoutadyen.CheckoutContext], vault myvault.VaultReader, publisher mypublisher.Publisher) (*service, error) {
	stripe.Key = apiKey
	return &service{
		apiKey:        apiKey,
		logger:        logger,
		nower:         nower,
		checkoutStore: checkoutStore,
		vault:         vault,
		publisher:     publisher,
	}, nil
}

// startCheckout starts a checkout session on the Adyen platform
func (s *service) startCheckout(c context.Context, basketUID string, returnURL string, params stripe.CheckoutSessionParams) (string, error) {
	now := s.nower.Now()

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

	accessToken, exist, err := s.vault.Get(c, myvault.CurrentToken)
	if err != nil || !exist && accessToken.ProviderName != "stripe" {
		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using api key")
		stripe.Key = s.apiKey
	} else {
		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using access token")
		stripe.Key = accessToken.AccessToken
	}

	session, err := session.New(&params)
	if err != nil {
		return "", myerrors.NewInvalidInputError(fmt.Errorf("error creating session: %s", err))
	}

	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		// Store checkout context on basketUID because we need it later again
		err = s.checkoutStore.Put(c, basketUID, checkoutadyen.CheckoutContext{
			BasketUID:         basketUID,
			CreatedAt:         now,
			OriginalReturnURL: returnURL,
			ID:                session.ID,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err))
		}
		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Stored checkout on basket-uid %s", basketUID)

		// Store checkout indexed on sessionID so we can fetch it later when we receive a webhook
		err = s.checkoutStore.Put(c, session.ID, checkoutadyen.CheckoutContext{
			BasketUID:         basketUID,
			CreatedAt:         now,
			OriginalReturnURL: returnURL,
			ID:                session.ID,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err))
		}
		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Stored checkout on stripe-uid %s", session.ID)

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

func (s *service) webhookNotification(c context.Context, username, password string, event stripe.Event) error {
	// TODO check username+password to make sure notification originates from Adyen

	s.logger.Log(c, event.ID, mylog.SeverityInfo, "Webhook: status update event %s with ID: %s", event.Type, event.ID)

	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case "payment_intent.created":
		{
			var paymentIntent stripe.PaymentIntent
			err := json.Unmarshal(event.Data.Raw, &paymentIntent)
			if err != nil {
				return myerrors.NewInvalidInputError(fmt.Errorf("error parsing webhook %v JSON: %v", event.Type, err))
			}
			return s.handlePaymentIntentCreated(c, event.ID, paymentIntent)
		}
	case "payment_intent.succeeded":
		{
			var paymentIntent stripe.PaymentIntent
			err := json.Unmarshal(event.Data.Raw, &paymentIntent)
			if err != nil {
				return myerrors.NewInvalidInputError(fmt.Errorf("error parsing webhook %v JSON: %v", event.Type, err))
			}
			return s.handlePaymentIntentSucceeded(c, event.ID, paymentIntent)
		}
	case "payment_method.attached":
		{
			var paymentMethod stripe.PaymentMethod
			err := json.Unmarshal(event.Data.Raw, &paymentMethod)
			if err != nil {
				return myerrors.NewInvalidInputError(fmt.Errorf("error parsing webhook %v JSON: %v", event.Type, err))
			}
			return s.handlePaymentMethodAttached(c, event.ID, paymentMethod)
		}
	default:
		{
			fmt.Printf("unhandled event type: %v\n", event.Type)
		}
	}
	return nil
}

func (s *service) handlePaymentIntentCreated(c context.Context, uid string, paymentIntent stripe.PaymentIntent) error {
	return myerrors.NewNotImplementedError(fmt.Errorf("unhandled event %+v", paymentIntent))
}

func (s *service) handlePaymentIntentSucceeded(c context.Context, uid string, paymentIntent stripe.PaymentIntent) error {

	s.logger.Log(c, uid, mylog.SeverityInfo, "Webhook: status update event received on payment %s: %+v", uid, paymentIntent)

	now := s.nower.Now()

	var checkoutContext checkoutadyen.CheckoutContext
	var found bool
	var err error
	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		checkoutContext, found, err = s.checkoutStore.Get(c, uid)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", uid))
		}

		s.logger.Log(c, uid, mylog.SeverityInfo, "Webhook: Payment %s is related to basket %s", uid, checkoutContext.BasketUID)

		checkoutContext.PaymentMethod = "ideal" // TODO
		checkoutContext.WebhookStatus = "payment_intent.succeeded"
		checkoutContext.WebhookSuccess = "true"
		checkoutContext.LastModified = &now

		err = s.checkoutStore.Put(c, uid, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		err = s.checkoutStore.Put(c, checkoutContext.BasketUID, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
			ProviderName:  "adyen",
			CheckoutUID:   checkoutContext.BasketUID,
			Status:        "payment_intent.succeeded",
			Success:       true,
			PaymentMethod: checkoutContext.PaymentMethod,
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

func (s *service) handlePaymentMethodAttached(c context.Context, uid string, paymentMethod stripe.PaymentMethod) error {
	return myerrors.NewNotImplementedError(fmt.Errorf("unhandled event"))
}
