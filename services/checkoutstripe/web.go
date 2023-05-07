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
	"github.com/MarcGrol/shopbackend/services/checkoutadyen"
)

type webService struct {
	logger  mylog.Logger
	service *service
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewWebService(apiKey string, nower mytime.Nower, checkoutStore mystore.Store[checkoutadyen.CheckoutContext], vault myvault.VaultReader, publisher mypublisher.Publisher) (*webService, error) {
	logger := mylog.New("checkoutstripe")
	s, err := newService(apiKey, logger, nower, checkoutStore, vault, publisher)
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

// startCheckoutPage starts a checkout session on the Adyen platform
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

	err := r.ParseForm()
	if err != nil {
		return stripe.CheckoutSessionParams{}, basketUID, "", myerrors.NewInvalidInputError(err)
	}

	returnURL := r.Form.Get("returnUrl")
	//	countryCode := r.Form.Get("countryCode")
	currency := r.Form.Get("currency")
	//amount, err := strconv.Atoi(r.Form.Get("amount"))
	if err != nil {
		return stripe.CheckoutSessionParams{}, basketUID, returnURL, myerrors.NewInvalidInputError(fmt.Errorf("invalid amount '%s' (%s)", r.Form.Get("amount"), err))
	}
	// addressCity := r.Form.Get("shopper.address.city")
	// addressCountry := r.Form.Get("shopper.address.country")
	// addressHouseNumber := r.Form.Get("shopper.address.houseNumber")
	// addressPostalCode := r.Form.Get("shopper.address.postalCode")
	// addressStateOrProvince := r.Form.Get("shopper.address.state")
	// addressStreet := r.Form.Get("shopper.address.street")
	shopperEmail := r.Form.Get("shopper.email")
	// companyHomepage := r.Form.Get("company.homepage")
	// companyName := r.Form.Get("company.name")
	// shopName := r.Form.Get("shop.name")

	// shopperDateOfBirth := func() *time.Time {
	// 	dob := r.Form.Get("shopper.dateOfBirth")
	// 	if dob == "" {
	// 		return nil
	// 	}
	// 	t, err := time.Parse("2006-01-02", r.Form.Get("shopper.dateOfBirth"))
	// 	if err != nil {
	// 		return nil
	// 	}
	// 	return &t
	// }()
	shopperLocale := r.Form.Get("shopper.locale")
	// shopperFirstName := r.Form.Get("shopper.firstName")
	// shopperLastName := r.Form.Get("shopper.lastName")
	//shopperUID := r.Form.Get("shopper.uid")
	// shopperPhoneNumber := r.Form.Get("shopper.phone")

	//expiresAt := time.Now().Add(time.Hour * 24)

	return stripe.CheckoutSessionParams{
		SuccessURL:        stripe.String(myhttp.HostnameWithScheme(r) + fmt.Sprintf("/stripe/checkout/%s/status/success", basketUID)),
		CancelURL:         stripe.String(myhttp.HostnameWithScheme(r) + fmt.Sprintf("/stripe/checkout/%s/status/cancel", basketUID)),
		ClientReferenceID: stripe.String(basketUID),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Tennis shoes"),
						Description: stripe.String("Ascis Gel Lyte 3"),
					},
					UnitAmount: stripe.Int64(int64(12000)),
				},
				Quantity: stripe.Int64(1),
			},
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Tennis racket"),
						Description: stripe.String("Bobolat Pure Strike 98"),
					},
					UnitAmount: stripe.Int64(int64(23000)),
				},
				Quantity: stripe.Int64(1),
			},
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Tennis balls"),
						Description: stripe.String("Dunlop Fort All Court"),
					},
					UnitAmount: stripe.Int64(int64(1000)),
				},
				Quantity: stripe.Int64(3),
			},
		},
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
		Currency:           stripe.String(currency),
		CustomerEmail:      stripe.String(shopperEmail),
		Locale:             stripe.String(shopperLocale),
		PaymentMethodTypes: stripe.StringSlice([]string{"ideal", "card"}),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"basketUID": basketUID,
			},
		},
	}, basketUID, returnURL, nil

}
