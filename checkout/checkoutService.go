package checkout

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
)

//go:embed templates
var templateFolder embed.FS
var (
	checkoutPageTemplate *template.Template
)

func init() {
	checkoutPageTemplate = template.Must(template.ParseFS(templateFolder, "templates/checkout.html"))
}

type Config struct {
	Environment     string
	MerchantAccount string
	ClientKey       string
	ApiKey          string
}

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
func NewService(cfg Config, payer Payer, checkoutStore mystore.Store[checkoutmodel.CheckoutContext], queue myqueue.TaskQueuer, nower mytime.Nower, logger mylog.Logger) (*service, error) {
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
		checkoutSessionResp, err := s.payer.Sessions(c, sessionRequest)
		if err != nil {
			errorWriter.WriteError(c, w, 2, fmt.Errorf("error creating payment session for checkout %s: %s", basketUID, err))
			return
		}

		// Ask the Adyen platform to return payment methods that are allowed for me
		paymentMethodsResp, err := s.payer.PaymentMethods(c, checkoutToPaymentMethodsRequest(sessionRequest))
		if err != nil {
			errorWriter.WriteError(c, w, 3, fmt.Errorf("error fetching payment methods for checkout %s: %s", basketUID, err))
			return
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
			PaymentMethodsResponse: *paymentMethodsResp,
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
			errorWriter.WriteError(c, w, 1, err)
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

		if len(event.NotificationItems) == 0 {
			s.logger.Log(c, event.NotificationItems[0].NotificationRequestItem.MerchantReference, mylog.SeverityInfo, "Webhook: status update on checkout received: %+v", event)
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
		return nil, basketUID, returnURL, myerrors.NewInvalidInputError(fmt.Errorf("invalid amount '%s' (%s)", r.Form.Get("amount"), err))
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

	//expiresAt := time.Now().Add(time.Hour * 24)

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
		BillingAddress: func() *checkout.Address {
			if addressCity != "" {
				return &checkout.Address{
					City:              addressCity,
					Country:           addressCountry,
					HouseNumberOrName: addressHouseNumber,
					PostalCode:        addressPostalCode,
					StateOrProvince:   addressStateOrProvince,
					Street:            addressStreet,
				}
			}
			return nil
		}(),
		//BlockedPaymentMethods: []string{},
		//CaptureDelayHours:     0,
		Channel: "Web",
		Company: func() *checkout.Company {
			if companyName != "" || companyHomepage != "" {
				return &checkout.Company{
					Homepage:           companyHomepage,
					Name:               companyName,
					RegistrationNumber: "",
					RegistryLocation:   "",
					TaxId:              "",
					Type:               "",
				}
			}
			return nil
		}(),
		CountryCode: countryCode,
		DateOfBirth: shopperDateOfBirth,
		//DeliverAt:   nil,
		DeliveryAddress: func() *checkout.Address {
			if addressCity != "" {
				return &checkout.Address{
					City:              addressCity,
					Country:           addressCountry,
					HouseNumberOrName: addressHouseNumber,
					PostalCode:        addressPostalCode,
					StateOrProvince:   addressStateOrProvince,
					Street:            addressStreet,
				}
			}
			return nil
		}(),
		//EnableOneClick:           false,
		//EnablePayOut:             false,
		//EnableRecurring:          false,
		//ExpiresAt: &expiresAt,
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
		ShopperName: func() *checkout.Name {
			if shopperFirstName != "" {
				return &checkout.Name{
					FirstName: shopperFirstName,
					LastName:  shopperLastName,
				}
			}
			return nil
		}(),
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
