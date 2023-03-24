package checkout

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/checkout/store"
	"github.com/MarcGrol/shopbackend/mycontext"
	"github.com/MarcGrol/shopbackend/myerrors"
	"github.com/MarcGrol/shopbackend/myhttp"
	"github.com/MarcGrol/shopbackend/mylog"
	"github.com/MarcGrol/shopbackend/shop/myqueue"
	"github.com/adyen/adyen-go-api-library/v6/src/adyen"
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"github.com/adyen/adyen-go-api-library/v6/src/common"
)

const (
	merchantAccountVarname = "ADYEN_MERCHANT_ACCOUNT"
	apiKeyVarname          = "ADYEN_API_KEY"
	clientKeyVarname       = "ADYEN_CLIENT_KEY"
	environmentVarname     = "ADYEN_ENVIRONMENT"
)

//go:embed templates
var templateFolder embed.FS
var (
	checkoutPageTemplate *template.Template
)

func init() {
	checkoutPageTemplate = template.Must(template.ParseFS(templateFolder, "templates/checkout.html"))
}

type service struct {
	environment     string
	merchantAccount string
	clientKey       string
	apiClient       *adyen.APIClient
	checkoutStore   store.CheckoutStorer
	queue           myqueue.TaskQueuer
	logger          mylog.Logger
}

type Starter interface {
	PrepareForCheckoutPage(basketUID string, req checkout.CreateCheckoutSessionRequest) (string, error)
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewService(checkoutStore store.CheckoutStorer, queue myqueue.TaskQueuer, logger mylog.Logger) (*service, error) {
	merchantAccount := os.Getenv(merchantAccountVarname)
	if merchantAccount == "" {
		return nil, myerrors.NewInvalidInputError(fmt.Errorf("missing env-var %s", merchantAccountVarname))
	}

	environment := os.Getenv(environmentVarname)
	if environment == "" {
		return nil, myerrors.NewInvalidInputError(fmt.Errorf("missing env-var %s", environmentVarname))
	}

	apiKey := os.Getenv(apiKeyVarname)
	if apiKey == "" {
		return nil, myerrors.NewInvalidInputError(fmt.Errorf("missing env-var %s", apiKeyVarname))
	}

	clientKey := os.Getenv(clientKeyVarname)
	if clientKey == "" {
		return nil, myerrors.NewInvalidInputError(fmt.Errorf("missing env-var %s", clientKeyVarname))
	}

	return &service{
		merchantAccount: merchantAccount,
		environment:     environment,
		clientKey:       clientKey,
		apiClient: adyen.NewClient(&common.Config{
			ApiKey:      apiKey,
			Environment: common.Environment(strings.ToUpper(environment)),
			//Debug:       true,
		}),
		checkoutStore: checkoutStore,
		queue:         queue,
		logger:        logger,
	}, nil
}

func (s service) RegisterEndpoints(c context.Context, router *mux.Router) {
	// Endpoints that compose the userinterface
	router.HandleFunc("/checkout/{basketUID}", s.startCheckoutPage()).Methods("POST")
	router.HandleFunc("/checkout/{basketUID}", s.finalizeCheckoutPage()).Methods("GET")
	router.HandleFunc("/checkout/{basketUID}/status/{status}", s.statusRedirectCallback()).Methods("GET")

	// Webhook notification called by Adyen at a later time
	router.HandleFunc("/checkout/webhook/event", s.webhookNotification()).Methods("POST")
}

// startCheckoutPage starts a checkout session on the Adyen platform
func (s service) startCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		// Convert request-body into a CreateCheckoutSessionRequest
		sessionRequest, basketUID, returnURL, err := parseRequest(r, s.merchantAccount)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("error parsing request: %s", err)))
			return
		}

		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Start checkout for basket %s", basketUID)

		// Initiate a checkout session on the Adyen platform
		checkoutSessionResp, _, err := s.apiClient.Checkout.Sessions(sessionRequest)
		if err != nil {
			errorWriter.WriteError(c, w, 2, fmt.Errorf("error creating payment session for checkout %s: %s", basketUID, err))
			return
		}

		// Ask the Adyen platform to return payment methods that are allowed for me
		paymentMethodsResp, _, err := s.apiClient.Checkout.PaymentMethods(checkoutToPaymentMethodsRequest(sessionRequest))
		if err != nil {
			errorWriter.WriteError(c, w, 3, fmt.Errorf("error fetching payment methods for checkoutContext %s: %s", basketUID, err))
			return
		}

		// Store checkout context because we need it later again
		err = s.checkoutStore.Put(c, basketUID, &checkoutmodel.CheckoutContext{
			BasketUID:         basketUID,
			OriginalReturnURL: returnURL,
			ID:                checkoutSessionResp.Id,
			SessionData:       checkoutSessionResp.SessionData,
		})
		if err != nil {
			errorWriter.WriteError(c, w, 4, myerrors.NewInternalError(fmt.Errorf("error storing checkout: %s", err)))
			return
		}

		// Pass relevant data to the checkout page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = checkoutPageTemplate.Execute(w, checkoutmodel.CheckoutPageInfo{
			Environment:     s.environment,
			ClientKey:       s.clientKey,
			MerchantAccount: s.merchantAccount,
			BasketUID:       basketUID,
			Amount: checkoutmodel.Amount{
				Currency: sessionRequest.Amount.Currency,
				Value:    sessionRequest.Amount.Value,
			},
			CountryCode:            sessionRequest.CountryCode,
			ShopperLocale:          sessionRequest.ShopperLocale,
			ShopperEmail:           sessionRequest.ShopperEmail,
			PaymentMethodsResponse: paymentMethodsResp,
			ID:                     checkoutSessionResp.Id,
			SessionData:            checkoutSessionResp.SessionData,
		})
		if err != nil {
			errorWriter.WriteError(c, w, 5, myerrors.NewInternalError(fmt.Errorf("error executimng template: %s", err)))
			return
		}
	}
}

// finalizeCheckoutPage is called when the shopper has finished the checkout process
func (s service) finalizeCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]

		checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
		if err != nil {
			errorWriter.WriteError(c, w, 10, myerrors.NewInternalError(err))
			return
		}
		if !found {
			errorWriter.WriteError(c, w, 11, myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID)))
			return
		}

		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Resume checkout for basket %s", basketUID)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Second time, less data is needed
		err = checkoutPageTemplate.Execute(w, checkoutmodel.CheckoutPageInfo{
			Environment:     s.environment,
			MerchantAccount: s.merchantAccount,
			ClientKey:       s.clientKey,
			BasketUID:       basketUID,
			ID:              checkoutContext.ID,
			SessionData:     checkoutContext.SessionData,
		})
		if err != nil {
			errorWriter.WriteError(c, w, 12, myerrors.NewInternalError(fmt.Errorf("error executimng template: %s", err)))
			return
		}
	}
}

func (s service) statusRedirectCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]
		status := mux.Vars(r)["status"]

		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Checkout completed for %s -> %s", basketUID, status)

		// TODO use a transaction here

		checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(fmt.Errorf("error fetching checkout with uid %s: %s", basketUID, err)))
			return
		}
		if !found {
			errorWriter.WriteError(c, w, 1, myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID)))
			return
		}

		checkoutContext.Status = status
		err = s.checkoutStore.Put(c, basketUID, &checkoutContext)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}

		adjustedReturnURL, err := addStatusQueryParam(checkoutContext.OriginalReturnURL, status)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(err))
			return
		}

		http.Redirect(w, r, adjustedReturnURL, http.StatusSeeOther)
	}
}

func addStatusQueryParam(orgUrl string, status string) (string, error) {
	u, err := url.Parse(orgUrl)
	if err != nil {
		return "", myerrors.NewInvalidInputError(fmt.Errorf("error parsing return URL %s: %s", orgUrl, err))
	}
	params := u.Query()
	params.Set("status", status)
	u.RawQuery = params.Encode()
	return u.String(), nil
}

func (s service) webhookNotification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		event := checkoutmodel.WebhookNotification{}
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			errorWriter.WriteError(c, w, 1, fmt.Errorf("error parsing webhook notification event:%s", err))
			return
		}

		for _, item := range event.NotificationItems {
			err := s.processNotificationItem(c, item)
			if err != nil {
				errorWriter.WriteError(c, w, 1, fmt.Errorf("error handling item: %s", err))
				return
			}
		}

		errorWriter.Write(c, w, http.StatusOK, checkoutmodel.WebhookNotificationResponse{
			Status: "[accepted]",
		})
	}
}

func (s service) processNotificationItem(c context.Context, item checkoutmodel.NotificationItem) error {
	basketUID := item.NotificationRequestItem.MerchantReference

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Webhook notification event received: %+v", item)

	// TODO use a transaction here

	checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
	if err != nil {
		return myerrors.NewInternalError(err)
	}
	if !found {
		return myerrors.NewNotFoundError(fmt.Errorf("checkout with uid %s not found", basketUID))
	}
	checkoutContext.PaymentMethod = item.NotificationRequestItem.PaymentMethod
	checkoutContext.WebhookStatus = item.NotificationRequestItem.EventCode
	checkoutContext.WebhookSuccess = item.NotificationRequestItem.Success

	err = s.checkoutStore.Put(c, basketUID, &checkoutContext)
	if err != nil {
		return myerrors.NewInternalError(err)
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

	// This could be where a basket is being converted into an order

	return nil
}

func parseRequest(r *http.Request, merchantAccount string) (*checkout.CreateCheckoutSessionRequest, string, string, error) {
	basketUID := mux.Vars(r)["basketUID"]
	if basketUID == "" {
		return nil, "", "", myerrors.NewInvalidInputError(fmt.Errorf("missing basketUID:%s", basketUID))
	}

	err := r.ParseForm()
	if err != nil {
		return nil, basketUID, "", myerrors.NewInvalidInputError(err)
	}

	returnURL := r.Form.Get("returnUrl")
	countryCode := r.Form.Get("countryCode")
	currency := r.Form.Get("currency")
	amount, err := strconv.Atoi(r.Form.Get("amount"))
	if err != nil {
		return nil, basketUID, returnURL, myerrors.NewInvalidInputError(fmt.Errorf("invalid amount:%s", err))
	}
	addressCity := r.Form.Get("shopper.address.city")
	addressCountry := r.Form.Get("shopper.address.country")
	addressHouseNumber := r.Form.Get("shopper.address.houseNumber")
	addressPostalCode := r.Form.Get("shopper.address.postalCode")
	addressStateOrProvince := r.Form.Get("shopper.address.state")
	addressStreet := r.Form.Get("shopper.address.street")
	shopperEmail := r.Form.Get("shopper.email")
	companyHomepage := r.Form.Get("company.homepage")
	companyName := r.Form.Get("company.name")
	// TODO: Understand why this field causes /session to fail
	//shopName := r.Form.Get("shop.name")

	shopperDateOfBirth := func() *time.Time {
		dob := r.Form.Get("shopper.dateOfBirth")
		if dob == "" {
			return nil
		}
		t, err := time.Parse("2006-01-02", r.Form.Get("shopper.dateOfBirth"))
		if err != nil {
			return nil
		}
		return &t
	}()
	shopperLocale := r.Form.Get("shopper.locale")
	shopperFirstName := r.Form.Get("shopper.firstName")
	shopperLastName := r.Form.Get("shopper.lastName")
	shopperUID := r.Form.Get("shopper.uid")
	shopperPhoneNumber := r.Form.Get("shopper.phone")

	expiresAt := time.Now().Add(time.Hour * 24)

	return &checkout.CreateCheckoutSessionRequest{
		//AccountInfo:           nil,
		//AdditionalAmount:      nil,
		//AdditionalData:        nil,
		AllowedPaymentMethods: []string{"ideal", "scheme"},
		Amount: checkout.Amount{
			Currency: currency,
			Value:    int64(amount),
		},
		//ApplicationInfo:    nil,
		//AuthenticationData: nil,
		BillingAddress: &checkout.Address{
			City:              addressCity,
			Country:           addressCountry,
			HouseNumberOrName: addressHouseNumber,
			PostalCode:        addressPostalCode,
			StateOrProvince:   addressStateOrProvince,
			Street:            addressStreet,
		},
		//BlockedPaymentMethods: []string{},
		//CaptureDelayHours:     0,
		Channel: "Web",
		Company: &checkout.Company{
			Homepage:           companyHomepage,
			Name:               companyName,
			RegistrationNumber: "",
			RegistryLocation:   "",
			TaxId:              "",
			Type:               "",
		},
		CountryCode: countryCode,
		DateOfBirth: shopperDateOfBirth,
		//DeliverAt:   nil,
		DeliveryAddress: &checkout.Address{
			City:              addressCity,
			Country:           addressCountry,
			HouseNumberOrName: addressHouseNumber,
			PostalCode:        addressPostalCode,
			StateOrProvince:   addressStateOrProvince,
			Street:            addressStreet,
		},
		//EnableOneClick:           false,
		//EnablePayOut:             false,
		//EnableRecurring:          false,
		ExpiresAt: &expiresAt,
		//LineItems:                nil,
		//Mandate:                  nil,
		//Mcc:                      "",
		MerchantAccount:        merchantAccount,
		MerchantOrderReference: basketUID,
		//Metadata:                 nil,
		//MpiData:                  nil,
		//RecurringExpiry:          "",
		//RecurringFrequency:       "",
		//RecurringProcessingModel: "",
		//RedirectFromIssuerMethod: "",
		//RedirectToIssuerMethod:   "",
		Reference:          basketUID,
		RiskData:           nil,
		ReturnUrl:          fmt.Sprintf("%s/checkout/%s", myhttp.HostnameWithScheme(r), basketUID),
		ShopperEmail:       shopperEmail,
		ShopperIP:          "",
		ShopperInteraction: "",
		ShopperLocale:      shopperLocale,
		ShopperName: &checkout.Name{
			FirstName: shopperFirstName,
			LastName:  shopperLastName,
		},
		ShopperReference: shopperUID,
		//ShopperStatement:          "",
		//SocialSecurityNumber:      "",
		//SplitCardFundingSources:   false,
		//Splits:                    nil,
		//Store: shopName,
		//StorePaymentMethod:        false,
		TelephoneNumber: shopperPhoneNumber,
		//ThreeDSAuthenticationOnly: false,
		TrustedShopper: true,
	}, basketUID, returnURL, nil
}

func checkoutToPaymentMethodsRequest(checkoutReq *checkout.CreateCheckoutSessionRequest) *checkout.PaymentMethodsRequest {
	return &checkout.PaymentMethodsRequest{
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
