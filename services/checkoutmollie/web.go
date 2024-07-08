package checkoutmollie

import (
	"context"
	"fmt"
	"net/http"

	"github.com/VictorAvelar/mollie-api-go/v3/mollie"
	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthvault"
)

type webService struct {
	logger  mylog.Logger
	service *service
	nower   mytime.Nower
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewWebService(apiKey string, payer Payer, nower mytime.Nower, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader[oauthvault.Token], publisher mypublisher.Publisher) (*webService, error) {
	logger := mylog.New("checkoutmollie")
	s, err := newService(apiKey, payer, logger, nower, checkoutStore, vault, publisher)
	if err != nil {
		return nil, err
	}

	return &webService{
		logger:  logger,
		service: s,
		nower:   nower,
	}, nil
}

func (s *webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	router.HandleFunc("/mollie/checkout/{basketUID}", s.startCheckoutPage()).Methods("POST")
	router.HandleFunc("/mollie/checkout/{basketUID}/status/{status}", s.checkoutCompletedPage()).Methods("GET")

	router.HandleFunc("/mollie/checkout/webhook/event/{basketUID}", s.webhookNotification()).Methods("POST")

	return nil
}

// startCheckoutPage starts a checkout session on the Mollie platform
func (s *webService) startCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		// Convert request-body into a CreateCheckoutSessionRequest
		params, basketUID, returnURL, err := s.parseRequest(r)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("error parsing request: %s", err)))
			return
		}

		redirectURL, err := s.service.startCheckout(c, basketUID, returnURL, params)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("error starting checkout: %s", err)))
			return
		}

		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}

func (s *webService) checkoutCompletedPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]
		status := mux.Vars(r)["status"]

		redirectURL, err := s.service.finalizeCheckout(c, basketUID, status)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("error starting checkout: %s", err)))
			return
		}

		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}

func (s *webService) webhookNotification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		username, password, _ := r.BasicAuth()
		basketUID := mux.Vars(r)["basketUID"]

		err := r.ParseForm()
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(err))
			return
		}

		id := r.FormValue("id")
		if id == "" {
			errorWriter.WriteError(c, w, 2, myerrors.NewInvalidInputErrorf("missing id"))
			return
		}

		err = s.service.webhookNotification(c, username, password, basketUID, id)
		if err != nil {
			errorWriter.WriteError(c, w, 4, err)
			return
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{})
	}
}

func (s *webService) parseRequest(r *http.Request) (mollie.Payment, string, string, error) {
	basketUID := mux.Vars(r)["basketUID"]
	if basketUID == "" {
		return mollie.Payment{}, "", "", myerrors.NewInvalidInputError(fmt.Errorf("missing basketUID:%s", basketUID))
	}

	co, err := checkoutapi.NewFromRequest(r)
	if err != nil {
		return mollie.Payment{}, "", "", myerrors.NewInvalidInputError(fmt.Errorf("error parsing form: %s", err))
	}

	paymentRequest := mollie.Payment{
		//IsCancellable: true,
		//TestMode:     true,
		DigitalGoods: false,
		//ApplePayPaymentToken
		BillingEmail: co.Shopper.ContactInfo.Email,
		//CardToken
		//Issuer
		//VoucherNumber
		//VoucherPin
		//ExtraMerchantData: co.BasketUID,
		//SessionID
		CustomerReference: co.Shopper.UID,
		ConsumerName:      co.Shopper.FirstName + " " + co.Shopper.LastName,
		//ConsumerAccount
		WebhookURL: fmt.Sprintf("%s/mollie/checkout/webhook/event/%s", myhttp.HostnameWithScheme(r),
			co.BasketUID),
		//Resource
		//ID: co.BasketUID,
		//MandateID
		//OrderID:   co.BasketUID,
		//ProfileID: "pfl_Ns8niaVZaw",
		//SettlementID
		//CustomerID: co.Shopper.UID,
		//Status
		Description: "Goods ordered in basket " + co.BasketUID,
		RedirectURL: fmt.Sprintf("%s/mollie/checkout/%s/status/success", myhttp.HostnameWithScheme(r), co.BasketUID),
		//CountryCode: "NL",
		//SubscriptionID
		CancelURL: fmt.Sprintf("%s/mollie/checkout/%s/status/cancelled", myhttp.HostnameWithScheme(r), co.BasketUID),
		Metadata: map[string]string{
			"basketUID": co.BasketUID,
		},
		Amount: &mollie.Amount{
			Currency: "EUR",
			Value:    fmt.Sprintf("%.2f", float32(co.TotalAmount.Value)/100.0),
		},
		//AmountRefunded
		//AmountRemaining
		//AmountCaptured
		//AmountChargedBack
		//SettlementAmount
		//ApplicationFee
		//Details: &mollie.PaymentDetails{},
		//CreatedAt: func() *time.Time { t := s.nower.Now(); return &t }(),
		//AuthorizedAt
		//PaidAt
		//CanceledAt
		//ExpiresAt
		//ExpiredAt
		//FailedAt
		//DueDate
		BillingAddress: &mollie.Address{
			StreetAndNumber: co.Shopper.Address.Street + " " + co.Shopper.Address.AddressHouseNumber,
			City:            co.Shopper.Address.City,
			Region:          co.Shopper.Address.State,
			PostalCode:      co.Shopper.Address.PostalCode,
			Country:         co.Shopper.Address.Country,
		},
		ShippingAddress: &mollie.PaymentDetailsAddress{
			StreetAndNumber: co.Shopper.Address.Street + " " + co.Shopper.Address.AddressHouseNumber,
			City:            co.Shopper.Address.City,
			Region:          co.Shopper.Address.State,
			PostalCode:      co.Shopper.Address.PostalCode,
			Country:         co.Shopper.Address.Country,
		},
		//Mode:   mollie.TestMode,
		Locale: "nl_NL",
		//RestrictPaymentMethodsToCountry
		//Method: []PaymentMethod{"ideal", "paypal"},
		//Links
		// SequenceType
	}

	return paymentRequest, basketUID, co.ReturnURL, nil

}
