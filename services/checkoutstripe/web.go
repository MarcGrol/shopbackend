package checkoutstripe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/stripe/stripe-go/v74"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
)

type webService struct {
	logger  mylog.Logger
	service *service
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewWebService(apiKey string, payer Payer, nower mytime.Nower, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader, publisher mypublisher.Publisher) (*webService, error) {
	logger := mylog.New("checkoutstripe")
	s, err := newService(apiKey, payer, logger, nower, checkoutStore, vault, publisher)
	if err != nil {
		return nil, err
	}

	return &webService{
		logger:  logger,
		service: s,
	}, nil
}

func (s *webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	router.HandleFunc("/stripe/checkout/{basketUID}", s.startCheckoutPage()).Methods("POST")
	router.HandleFunc("/stripe/checkout/{basketUID}/status/{status}", s.checkoutCompletedPage()).Methods("GET")

	router.HandleFunc("/stripe/checkout/webhook/event", s.webhookNotification()).Methods("POST")

	return nil
}

// startCheckoutPage starts a checkout session on the Stripe platform
func (s *webService) startCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		// Convert request-body into a CreateCheckoutSessionRequest
		params, basketUID, returnURL, err := parseRequest(r)
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

		event := stripe.Event{}
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		err = s.service.webhookNotification(c, username, password, event)
		if err != nil {
			errorWriter.WriteError(c, w, 2, err)
			return
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{})
	}
}

func parseRequest(r *http.Request) (stripe.CheckoutSessionParams, string, string, error) {
	basketUID := mux.Vars(r)["basketUID"]
	if basketUID == "" {
		return stripe.CheckoutSessionParams{}, "", "", myerrors.NewInvalidInputError(fmt.Errorf("missing basketUID:%s", basketUID))
	}

	co, err := checkoutapi.NewFromRequest(r)
	if err != nil {
		return stripe.CheckoutSessionParams{}, "", "", myerrors.NewInvalidInputError(fmt.Errorf("error parsing form: %s", err))
	}

	return stripe.CheckoutSessionParams{
		Params: stripe.Params{
			Metadata: map[string]string{
				"basketUID": basketUID,
			},
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"basketUID": basketUID, // This is to correlare the webhook with the basket
			},
			Shipping: &stripe.ShippingDetailsParams{
				Name:           stripe.String(co.Shopper.FirstName + " " + co.Shopper.LastName),
				Phone:          stripe.String(co.Shopper.ContactInfo.PhoneNumber),
				TrackingNumber: stripe.String(basketUID),
				Address: &stripe.AddressParams{
					City:       stripe.String(co.Shopper.Address.City),
					Country:    stripe.String(co.Shopper.Address.Country),
					Line1:      stripe.String(co.Shopper.Address.Street + " " + co.Shopper.Address.AddressHouseNumber),
					PostalCode: stripe.String(co.Shopper.Address.PostalCode),
				},
			},
		},
		SuccessURL:        stripe.String(myhttp.HostnameWithScheme(r) + fmt.Sprintf("/stripe/checkout/%s/status/success", basketUID)),
		CancelURL:         stripe.String(myhttp.HostnameWithScheme(r) + fmt.Sprintf("/stripe/checkout/%s/status/cancel", basketUID)),
		ClientReferenceID: stripe.String(basketUID),
		LineItems: func() []*stripe.CheckoutSessionLineItemParams {
			products := []*stripe.CheckoutSessionLineItemParams{}
			for _, p := range co.Products {
				products = append(products, &stripe.CheckoutSessionLineItemParams{
					PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
						Currency: stripe.String(p.Currency),
						ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
							Name:        stripe.String(p.Name),
							Description: stripe.String(p.Description),
						},
						UnitAmount: stripe.Int64(int64(p.ItemPrice)),
					},
					Quantity: stripe.Int64(int64(p.Quantity)),
				})
			}
			return products
		}(),
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
		Currency:           stripe.String(co.TotalAmount.Currency),
		CustomerEmail:      stripe.String(co.Shopper.ContactInfo.Email),
		Locale:             stripe.String(co.Shopper.Locale),
		PaymentMethodTypes: stripe.StringSlice([]string{"ideal", "card"}),
	}, basketUID, co.ReturnURL, nil

}
