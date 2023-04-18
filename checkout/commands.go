package checkout

import (
	"context"
	"fmt"
	"net/url"

	"github.com/adyen/adyen-go-api-library/v6/src/checkout"

	"github.com/MarcGrol/shopbackend/checkout/checkoutevents"
	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/myvault"
)

func (s *service) CreateTopics(c context.Context) error {
	client, cleanup, err := mypubsub.New(c)
	if err != nil {
		return fmt.Errorf("error creating client: %s", err)
	}
	defer cleanup()

	err = client.CreateTopic(c, checkoutevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", checkoutevents.TopicName, err)
	}

	return nil
}

// startCheckout starts a checkout session on the Adyen platform
func (s *service) startCheckout(c context.Context, basketUID string, req checkout.CreateCheckoutSessionRequest, returnURL string) (*checkoutmodel.CheckoutPageInfo, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

	req.MerchantAccount = s.merchantAccount
	err := validateRequest(req)
	if err != nil {
		return nil, myerrors.NewInvalidInputError(err)
	}

	now := s.nower.Now()

	var paymentMethodsResp checkout.PaymentMethodsResponse
	var checkoutSessionResp checkout.CreateCheckoutSessionResponse
	err = s.checkoutStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		accessToken, exist, err := s.vault.Get(c, myvault.CurrentToken)
		if err != nil || !exist {
			s.payer.UseApiKey(s.apiKey)
			s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using api-key")
		} else {
			s.payer.UseToken(accessToken.AccessToken)
			s.logger.Log(c, basketUID, mylog.SeverityInfo, "Using access token")
		}

		// Initiate a checkout session on the Adyen platform
		checkoutSessionResp, err = s.payer.Sessions(c, req)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error creating payment session for checkout %s: %s", basketUID, err))
		}

		// Ask the Adyen platform to return payment methods that are allowed for this merchant
		paymentMethodsResp, err = s.payer.PaymentMethods(c, checkoutToPaymentMethodsRequest(req))
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error fetching payment methods for checkout %s: %s", basketUID, err))
		}

		// Store checkout context because we need it later again
		err = s.checkoutStore.Put(c, basketUID, checkoutmodel.CheckoutContext{
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
		req.Reference == "" || req.MerchantAccount == "" || req.Channel == "" {
		return myerrors.NewInvalidInputError(fmt.Errorf("Missing mandatory field"))
	}

	return nil
}

// resumeCheckout is called when the shopper has finished the checkout process
func (s *service) resumeCheckout(c context.Context, basketUID string) (*checkoutmodel.CheckoutPageInfo, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Resume checkout for basket %s", basketUID)

	checkoutContext := checkoutmodel.CheckoutContext{}
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

	return &checkoutmodel.CheckoutPageInfo{
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

func (s *service) webhookNotification(c context.Context, username, password string, event checkoutmodel.WebhookNotification) error {

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

func (s *service) processNotificationItem(c context.Context, item checkoutmodel.NotificationItem) error {
	basketUID := item.NotificationRequestItem.MerchantReference

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Webhook: status update event received on basket %s: %+v", item.NotificationRequestItem.MerchantReference, item)

	now := s.nower.Now()

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
		checkoutContext.LastModified = &now

		err = s.checkoutStore.Put(c, basketUID, checkoutContext)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		err = s.publisher.Publish(c, checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
			CheckoutUID:   basketUID,
			Status:        item.NotificationRequestItem.EventCode,
			Success:       item.NotificationRequestItem.Success == "true",
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
