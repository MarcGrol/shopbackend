package checkoutadyen

import (
	"context"
	"fmt"
	"net/url"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthvault"
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
)

type service struct {
	environment     string
	merchantAccount string
	clientKey       string
	apiKey          string
	payer           Payer
	checkoutStore   mystore.Store[checkoutapi.CheckoutContext]
	vault           myvault.VaultReader[oauthvault.Token]
	nower           mytime.Nower
	logger          mylog.Logger
	subscriber      mypubsub.PubSub
	publisher       mypublisher.Publisher
}

// Use dependency injection to isolate the infrastructure and easy testing
func newCommandService(cfg Config, payer Payer, checkoutStorer mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader[oauthvault.Token], nower mytime.Nower, logger mylog.Logger, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) (*service, error) {
	return &service{
		merchantAccount: cfg.MerchantAccount,
		environment:     cfg.Environment,
		clientKey:       cfg.ClientKey,
		apiKey:          cfg.APIKey,
		payer:           payer,
		checkoutStore:   checkoutStorer,
		vault:           vault,
		nower:           nower,
		logger:          logger,
		subscriber:      subscriber,
		publisher:       publisher,
	}, nil
}

func (s *service) CreateTopics(c context.Context) error {
	err := s.publisher.CreateTopic(c, checkoutevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", checkoutevents.TopicName, err)
	}

	return nil
}

// startCheckout starts a checkout session on the Adyen platform
func (s *service) payByLink(c context.Context, basketUID string, req checkout.CreatePaymentLinkRequest, returnURL string) (string, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start pbl-checkout for basket %s", basketUID)

	req.MerchantAccount = s.merchantAccount
	err := validatePayByLinkRequest(req)
	if err != nil {
		return "", myerrors.NewInvalidInputError(err)
	}

	now := s.nower.Now()

	// Initiate a checkout session on the Adyen platform
	s.setupAuthentication(c, basketUID)
	resp, err := s.payer.CreatePayByLink(c, req)
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error creating pay-by-link for checkout %s: %s", basketUID, err))
	}

	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		// Store checkout context because we need it later again
		err = s.checkoutStore.Put(c, basketUID, checkoutapi.CheckoutContext{
			PaymentProvider:   "adyen",
			BasketUID:         basketUID,
			CreatedAt:         now,
			OriginalReturnURL: returnURL,
			ID:                resp.Id,
			PayByLink:         true,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err))
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.PayByLinkCreated{
			ProviderName:  "adyen",
			CheckoutUID:   basketUID,
			AmountInCents: req.Amount.Value,
			Currency:      req.Amount.Currency,
			ShopperUID:    req.ShopperEmail,
			MerchantUID:   req.MerchantAccount,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return resp.Url, nil
}

func validatePayByLinkRequest(req checkout.CreatePaymentLinkRequest) error {
	if req.Amount.Currency == "" || req.Amount.Value == 0 ||
		req.CountryCode == "" ||
		req.ShopperLocale == "" || req.ReturnUrl == "" || req.MerchantOrderReference == "" ||
		req.Reference == "" || req.MerchantAccount == "" {
		return myerrors.NewInvalidInputError(fmt.Errorf("missing mandatory field"))
	}

	return nil
}

// startCheckout starts a checkout session on the Adyen platform
func (s *service) startCheckout(c context.Context, basketUID string, req checkout.CreateCheckoutSessionRequest, returnURL string) (*CheckoutPageInfo, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

	req.MerchantAccount = s.merchantAccount
	err := validateRequest(req)
	if err != nil {
		return nil, myerrors.NewInvalidInputError(err)
	}

	now := s.nower.Now()

	// Initiate a checkout session on the Adyen platform
	s.setupAuthentication(c, basketUID)
	checkoutSessionResp, err := s.payer.Sessions(c, req)
	if err != nil {
		return nil, myerrors.NewInternalError(fmt.Errorf("error creating payment session for checkout %s: %s", basketUID, err))
	}

	// Ask the Adyen platform to return payment methods that are allowed for this merchant
	paymentMethodsResp, err := s.payer.PaymentMethods(c, checkoutToPaymentMethodsRequest(req))
	if err != nil {
		return nil, myerrors.NewInternalError(fmt.Errorf("error fetching payment methods for checkout %s: %s", basketUID, err))
	}

	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		// Store checkout context because we need it later again
		err = s.checkoutStore.Put(c, basketUID, checkoutapi.CheckoutContext{
			BasketUID:         basketUID,
			CreatedAt:         now,
			OriginalReturnURL: returnURL,
			ID:                checkoutSessionResp.Id,
			SessionData:       checkoutSessionResp.SessionData,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err))
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName:  "adyen",
			CheckoutUID:   basketUID,
			AmountInCents: req.Amount.Value,
			Currency:      req.Amount.Currency,
			ShopperUID:    req.ShopperEmail,
			MerchantUID:   req.MerchantAccount,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &CheckoutPageInfo{
		Environment:     s.environment,
		ClientKey:       s.clientKey,
		MerchantAccount: s.merchantAccount,
		BasketUID:       basketUID,
		Amount: Amount{
			Currency: req.Amount.Currency,
			Value:    req.Amount.Value,
		},
		CountryCode:            req.CountryCode,
		ShopperLocale:          req.ShopperLocale,
		ShopperEmail:           req.ShopperEmail,
		PaymentMethodsResponse: paymentMethodsResp,
		ID:                     checkoutSessionResp.Id,
		SessionData:            checkoutSessionResp.SessionData,
		Products: func() []Product {
			products := []Product{}
			for _, item := range *(req.LineItems) {
				products = append(products, Product{
					Name:        item.Id,
					Description: item.Description,
					ItemPrice: Amount{
						Currency: req.Amount.Currency,
						Value:    item.AmountIncludingTax,
					},
					Quantity: int(item.Quantity),
					TotalPrice: Amount{
						Currency: req.Amount.Currency,
						Value:    item.AmountIncludingTax * int64(item.Quantity),
					},
				})
			}
			return products
		}(),
		ShopperFullname: req.ShopperName.FirstName + " " + req.ShopperName.LastName,
	}, nil
}

func validateRequest(req checkout.CreateCheckoutSessionRequest) error {
	if req.Amount.Currency == "" || req.Amount.Value == 0 ||
		req.CountryCode == "" ||
		req.ShopperLocale == "" || req.ReturnUrl == "" || req.MerchantOrderReference == "" ||
		req.Reference == "" || req.MerchantAccount == "" || req.Channel == "" {
		return myerrors.NewInvalidInputError(fmt.Errorf("missing mandatory field"))
	}

	return nil
}

func (s *service) setupAuthentication(c context.Context, basketUID string) {
	tokenUID := oauthvault.CurrentToken + "_" + "adyen"
	accessToken, exist, err := s.vault.Get(c, tokenUID)
	if err != nil || !exist || accessToken.ProviderName != "adyen" ||
		accessToken.SessionUID == "" ||
		(accessToken.ExpiresIn != nil && accessToken.ExpiresIn.Before(s.nower.Now())) {
		s.payer.UseAPIKey(s.apiKey)
		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using api-key")
		return
	}

	s.payer.UseToken(accessToken.AccessToken)
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using access token")
}

// resumeCheckout is called when the shopper has finished the checkout process
func (s *service) resumeCheckout(c context.Context, basketUID string) (*CheckoutPageInfo, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Resume checkout for basket %s", basketUID)

	checkoutContext := checkoutapi.CheckoutContext{}

	var found bool
	var err error
	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		checkoutContext, found, err = s.checkoutStore.Get(c, basketUID)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &CheckoutPageInfo{
		Completed:       true,
		Environment:     s.environment,
		MerchantAccount: s.merchantAccount,
		ClientKey:       s.clientKey,
		BasketUID:       basketUID,
		ID:              checkoutContext.ID,
		SessionData:     checkoutContext.SessionData,
	}, nil
}

func (s *service) finalizeCheckout(c context.Context, basketUID string, status string) (string, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Redirect (start): Checkout completed for basket %s -> %s", basketUID, status)

	now := s.nower.Now()

	var checkoutContext checkoutapi.CheckoutContext
	var found bool
	var err error
	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		checkoutContext, found, err = s.checkoutStore.Get(c, basketUID)
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

		return nil
	})
	if err != nil {
		return "", err
	}

	adjustedReturnURL, err := addStatusQueryParam(checkoutContext.OriginalReturnURL, status)
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error adjusting url: %s", err))
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

func (s *service) webhookNotification(c context.Context, username, password string, event WebhookNotification) error {

	// TODO check username+password to make sure notification originates from Adyen

	if len(event.NotificationItems) >= 0 {
		s.logger.Log(c, event.NotificationItems[0].NotificationRequestItem.MerchantReference, mylog.SeverityInfo, "Webhook: status update on basket received: %+v", event)
	}

	for _, item := range event.NotificationItems {
		err := s.processNotificationItem(c, item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *service) processNotificationItem(c context.Context, item NotificationItem) error {
	basketUID := item.NotificationRequestItem.MerchantReference

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Webhook: status update event received on basket %s: %+v", item.NotificationRequestItem.MerchantReference, item)

	now := s.nower.Now()

	var checkoutContext checkoutapi.CheckoutContext
	var found bool
	var err error
	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		checkoutContext, found, err = s.checkoutStore.Get(c, basketUID)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID))
		}

		eventStatus := classifyEventStatus(item.NotificationRequestItem.EventCode, item.NotificationRequestItem.Success == "true")
		eventStatusDetails := fmt.Sprintf("%s=%s", item.NotificationRequestItem.EventCode, item.NotificationRequestItem.Success)

		checkoutContext.PaymentMethod = item.NotificationRequestItem.PaymentMethod
		checkoutContext.LastModified = &now
		checkoutContext.CheckoutStatus = eventStatus
		checkoutContext.CheckoutStatusDetails = eventStatusDetails

		err = s.checkoutStore.Put(c, basketUID, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
			ProviderName:          "adyen",
			CheckoutUID:           basketUID,
			PaymentMethod:         checkoutContext.PaymentMethod,
			CheckoutStatus:        eventStatus,
			CheckoutStatusDetails: eventStatusDetails,
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

func classifyEventStatus(eventName string, status bool) checkoutevents.CheckoutStatus {
	// https://docs.adyen.com/development-resources/webhooks/webhook-types#standard-webhook
	switch eventName {
	case "AUTHORISATION", "AUTHORISATION_ADJUSTMENT":
		if status {
			return checkoutevents.CheckoutStatusSuccess
		} else {
			return checkoutevents.CheckoutStatusFailed
		}
	case "PENDING":
		return checkoutevents.CheckoutStatusPending
	case "OFFER_CLOSED":
		return checkoutevents.CheckoutStatusExpired
	case "CANCELLATION":
		return checkoutevents.CheckoutStatusCancelled
	case "NOTIFICATION_OF_FRAUD":
		return checkoutevents.CheckoutStatusFraud
	default:
		// "CANCEL_OR_REFUND", "CAPTURE","HANDLED_EXTERNALLY", "ORDER_OPENED", "ORDER_CLOSED", "REFUND", "REFUND_FAILED",
		// "REFUNDED_REVERSED", "REFUND_WITH_DATA", "REPORT_AVAILABLE", "VOID_PENDING_REFUND":
		return checkoutevents.CheckoutStatusUndefined
	}
}
func checkoutToPaymentMethodsRequest(checkoutReq checkout.CreateCheckoutSessionRequest) checkout.PaymentMethodsRequest {
	return checkout.PaymentMethodsRequest{
		Channel:         "Web",
		MerchantAccount: checkoutReq.MerchantAccount,
		CountryCode:     checkoutReq.CountryCode,
		ShopperLocale:   checkoutReq.ShopperLocale,
		Amount: &checkout.Amount{
			Currency: checkoutReq.Amount.Currency,
			Value:    checkoutReq.Amount.Value,
		},
	}
}
