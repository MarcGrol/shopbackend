package checkoutstripe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/stripe/stripe-go/v74"

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
	stripe.Key = apiKey
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

func (s *service) startCheckout(c context.Context, basketUID string, returnURL string, params stripe.CheckoutSessionParams) (string, error) {
	now := s.nower.Now()

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

	// Iniitialize payment to the stripe platform
	s.setupAuthentication(c, basketUID)
	session, err := s.payer.CreateCheckoutSession(c, params)
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

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s completed", basketUID)

	return session.URL, nil
}

func (s *service) setupAuthentication(c context.Context, basketUID string) {
	tokenUID := myvault.CurrentToken + "_" + ("stripe")
	accessToken, exist, err := s.vault.Get(c, tokenUID)
	if err != nil || !exist || accessToken.ProviderName != "stripe" || accessToken.SessionUID == "" {
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

func (s *service) webhookNotification(c context.Context, username, password string, event stripe.Event) error {
	// TODO check username+password to make sure notification originates from Strip

	s.logger.Log(c, event.ID, mylog.SeverityInfo, "Webhook: status update event %s with ID: %s", event.Type, event.ID)

	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case "payment_intent.created", "payment_intent.succeeded", "payment_intent.canceled", "payment_intent.payment_failed":
		{
			var paymentIntent stripe.PaymentIntent
			err := json.Unmarshal(event.Data.Raw, &paymentIntent)
			if err != nil {
				return myerrors.NewInvalidInputError(fmt.Errorf("error parsing webhook %v JSON: %v", event.Type, err))
			}
			return s.handlePaymentIntentEvent(c, event.Type, paymentIntent)
		}

	case "payment_method.attached":
		{
			var paymentMethod stripe.PaymentMethod
			err := json.Unmarshal(event.Data.Raw, &paymentMethod)
			if err != nil {
				return myerrors.NewInvalidInputError(fmt.Errorf("error parsing webhook %v JSON: %v", event.Type, err))
			}
			return s.handlePaymentMethodEvent(c, event.Type, paymentMethod)
		}

		// TODO "payment_intent.partially_funded", "payment_intent.processing",
		//  "payment_intent.requires_action", "payment_intent.requires_capture",
	default:
		{
			fmt.Printf("unhandled event type: %v\n", event.Type)
		}
	}
	return nil
}

func (s *service) handlePaymentIntentEvent(c context.Context, eventType string, paymentIntent stripe.PaymentIntent) error {
	uid := paymentIntent.Metadata["basketUID"]

	s.logger.Log(c, uid, mylog.SeverityInfo, "Webhook: status update event received on payment %s: %+v", uid, paymentIntent)

	now := s.nower.Now()

	var checkoutContext checkoutapi.CheckoutContext
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

		s.logger.Log(c, uid, mylog.SeverityInfo, "Webhook: Payment event %s is related to basket %s", eventType, checkoutContext.BasketUID)

		checkoutContext.PaymentMethod = func() string {
			if len(paymentIntent.PaymentMethodTypes) == 0 {
				return ""
			}
			return paymentIntent.PaymentMethodTypes[0]
		}()
		checkoutContext.WebhookEventName = eventType
		checkoutContext.WebhookEventSuccess = (eventType == "payment_intent.succeeded")
		checkoutContext.LastModified = &now

		err = s.checkoutStore.Put(c, checkoutContext.BasketUID, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
			ProviderName:  "stripe",
			CheckoutUID:   checkoutContext.BasketUID,
			Status:        eventType,
			Success:       checkoutContext.WebhookEventSuccess,
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

func (s *service) handlePaymentMethodEvent(c context.Context, eventType string, paymentMethod stripe.PaymentMethod) error {
	return myerrors.NewNotImplementedError(fmt.Errorf("unhandled event"))
}
