package checkout

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/adyen/adyen-go-api-library/v6/src/checkout"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
)

type service struct {
	environment     string
	merchantAccount string
	clientKey       string
	apiKey          string
	payer           Payer
	checkoutStore   mystore.Store[checkoutmodel.CheckoutContext]
	queue           myqueue.TaskQueuer
	nower           mytime.Nower
	logger          mylog.Logger
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(cfg Config, payer Payer, checkoutStore mystore.Store[checkoutmodel.CheckoutContext], queue myqueue.TaskQueuer, nower mytime.Nower, logger mylog.Logger) (*service, error) {
	return &service{
		merchantAccount: cfg.MerchantAccount,
		environment:     cfg.Environment,
		clientKey:       cfg.ClientKey,
		apiKey:          cfg.ApiKey,
		payer:           payer,
		checkoutStore:   checkoutStore,
		queue:           queue,
		nower:           nower,
		logger:          logger,
	}, nil
}

// startCheckoutPage starts a checkout session on the Adyen platform
func (s service) startCheckoutPage(c context.Context, basketUID string, req checkout.CreateCheckoutSessionRequest, returnURL string) (*checkoutmodel.CheckoutPageInfo, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

	req.MerchantAccount = s.merchantAccount
	err := validateRequest(req)
	if err != nil {
		return nil, myerrors.NewInvalidInputError(err)
	}

	// Initiate a checkout session on the Adyen platform
	checkoutSessionResp, err := s.payer.Sessions(c, req)
	if err != nil {
		return nil, myerrors.NewInternalError(fmt.Errorf("error creating payment session for checkout %s: %s", basketUID, err))
	}

	// Ask the Adyen platform to return payment methods that are allowed for this merchant
	paymentMethodsResp, err := s.payer.PaymentMethods(c, checkoutToPaymentMethodsRequest(req))
	if err != nil {
		return nil, myerrors.NewInternalError(fmt.Errorf("error fetching payment methods for checkout %s: %s", basketUID, err))
	}

	// Store checkout context because we need it later again
	err = s.checkoutStore.Put(c, basketUID, checkoutmodel.CheckoutContext{
		BasketUID:         basketUID,
		CreatedAt:         s.nower.Now(),
		OriginalReturnURL: returnURL,
		ID:                checkoutSessionResp.Id,
		SessionData:       checkoutSessionResp.SessionData,
	})
	if err != nil {
		return nil, myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err))
	}

	return &checkoutmodel.CheckoutPageInfo{
		Environment:     s.environment,
		ClientKey:       s.clientKey,
		MerchantAccount: s.merchantAccount,
		BasketUID:       basketUID,
		Amount: checkoutmodel.Amount{
			Currency: req.Amount.Currency,
			Value:    req.Amount.Value,
		},
		CountryCode:            req.CountryCode,
		ShopperLocale:          req.ShopperLocale,
		ShopperEmail:           req.ShopperEmail,
		PaymentMethodsResponse: paymentMethodsResp,
		ID:                     checkoutSessionResp.Id,
		SessionData:            checkoutSessionResp.SessionData,
	}, nil
}

func validateRequest(req checkout.CreateCheckoutSessionRequest) error {
	if req.Amount.Currency == "" || req.Amount.Value == 0 || req.CountryCode == "" ||
		req.ShopperLocale == "" || req.ReturnUrl == "" || req.MerchantOrderReference == "" ||
		req.Reference == "" || req.MerchantAccount == "" {
		return myerrors.NewInvalidInputError(fmt.Errorf("Missing mandatory field"))
	}

	return nil
}

// finalizeCheckoutPage is called when the shopper has finished the checkout process
func (s service) finalizeCheckoutPage(c context.Context, basketUID string) (*checkoutmodel.CheckoutPageInfo, error) {
	checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
	if err != nil {
		return nil, myerrors.NewInternalError(err)
	}
	if !found {
		return nil, myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID))
	}

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Resume checkout for basket %s", basketUID)

	return &checkoutmodel.CheckoutPageInfo{
		Environment:     s.environment,
		MerchantAccount: s.merchantAccount,
		ClientKey:       s.clientKey,
		BasketUID:       basketUID,
		ID:              checkoutContext.ID,
		SessionData:     checkoutContext.SessionData,
	}, nil
}

func (s service) statusRedirectCallback(c context.Context, basketUID string, status string) (string, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Redirect: Checkout completed for checkout for %s -> %s", basketUID, status)

	var checkoutContext checkoutmodel.CheckoutContext
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
		checkoutContext.LastModified = func() *time.Time { t := s.nower.Now(); return &t }()

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

	return adjustedReturnURL, nil
}

func addStatusQueryParam(orgUrl string, status string) (string, error) {
	u, err := url.Parse(orgUrl)
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error parsing return URL %s: %s", orgUrl, err))
	}
	params := u.Query()
	params.Set("status", status)
	u.RawQuery = params.Encode()
	return u.String(), nil
}

func (s service) webhookNotification(c context.Context, event checkoutmodel.WebhookNotification) error {

	if len(event.NotificationItems) >= 0 {
		s.logger.Log(c, event.NotificationItems[0].NotificationRequestItem.MerchantReference, mylog.SeverityInfo, "Webhook: status update on checkout received: %+v", event)
	}

	for _, item := range event.NotificationItems {
		err := s.processNotificationItem(c, item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s service) processNotificationItem(c context.Context, item checkoutmodel.NotificationItem) error {
	basketUID := item.NotificationRequestItem.MerchantReference

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Webhook: status update event received: %+v", item)

	var checkoutContext checkoutmodel.CheckoutContext
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
		checkoutContext.PaymentMethod = item.NotificationRequestItem.PaymentMethod
		checkoutContext.WebhookStatus = item.NotificationRequestItem.EventCode
		checkoutContext.WebhookSuccess = item.NotificationRequestItem.Success
		checkoutContext.LastModified = func() *time.Time { t := s.nower.Now(); return &t }()

		err = s.checkoutStore.Put(c, basketUID, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Asynchronously inform basket service
	err = s.queue.Enqueue(c, myqueue.Task{
		UID: basketUID,
		WebhookURLPath: fmt.Sprintf("/api/basket/%s/status/%s/%s", basketUID,
			item.NotificationRequestItem.EventCode, item.NotificationRequestItem.Success),
		Payload: []byte{},
	})
	if err != nil {
		return myerrors.NewInternalError(fmt.Errorf("error queueing notification to basket %s: %s", basketUID, err))
	}

	return nil
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
