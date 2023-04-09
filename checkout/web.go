package checkout

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
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

type webService struct {
	logger  mylog.Logger
	service *service
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewService(cfg Config, payer Payer, checkoutStore mystore.Store[checkoutmodel.CheckoutContext], queuer myqueue.TaskQueuer, nower mytime.Nower) (*webService, error) {
	logger := mylog.New("checkout")
	s, err := newService(cfg, payer, checkoutStore, queuer, nower, logger)
	if err != nil {
		return nil, err
	}
	return &webService{
		logger:  logger,
		service: s,
	}, nil
}

func (s webService) RegisterEndpoints(c context.Context, router *mux.Router) {
	// Endpoints that compose the user-interface
	router.HandleFunc("/checkout/{basketUID}", s.startCheckoutPage()).Methods("POST")
	router.HandleFunc("/checkout/{basketUID}", s.resumeCheckoutPage()).Methods("GET")

	// Adyen will redirect to this endpoint after checkout has finalized
	router.HandleFunc("/checkout/{basketUID}/status/{status}", s.finalizeCheckoutPage()).Methods("GET")

	// Final notification called by Adyen at a later time
	router.HandleFunc("/checkout/webhook/event", s.webhookNotification()).Methods("POST")
}

// startCheckoutPage starts a checkout session on the Adyen platform
func (s webService) startCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		// Convert request-body into a CreateCheckoutSessionRequest
		sessionRequest, basketUID, returnURL, err := parseRequest(r)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("error parsing request: %s", err)))
			return
		}

		resp, err := s.service.startCheckout(c, basketUID, sessionRequest, returnURL)
		if err != nil {
			errorWriter.WriteError(c, w, 2, err)
			return
		}

		// Pass relevant data to the checkout page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = checkoutPageTemplate.Execute(w, resp)
		if err != nil {
			errorWriter.WriteError(c, w, 5, myerrors.NewInternalError(fmt.Errorf("error executing template: %s", err)))
			return
		}
	}
}

// resumeCheckoutPage is called halfway the checkout process
func (s webService) resumeCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]

		resp, err := s.service.resumeCheckout(c, basketUID)
		if err != nil {
			errorWriter.WriteError(c, w, 10, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Second time, less data is needed
		err = checkoutPageTemplate.Execute(w, resp)
		if err != nil {
			errorWriter.WriteError(c, w, 12, myerrors.NewInternalError(fmt.Errorf("error executing template: %s", err)))
			return
		}
	}
}

// finalizeCheckoutPage reports the status after finalisation of the checkout
func (s webService) finalizeCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]
		status := mux.Vars(r)["status"]

		redirectURL, err := s.service.finalizeCheckout(c, basketUID, status)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}

// webhookNotification received a json-formatted notification message with the definitive checkout status
func (s webService) webhookNotification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		event := checkoutmodel.WebhookNotification{}
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			errorWriter.WriteError(c, w, 1, fmt.Errorf("error parsing webhook notification event:%s", err))
			return
		}

		err = s.service.webhookNotification(c, event)
		if err != nil {
			errorWriter.Write(c, w, http.StatusOK, checkoutmodel.WebhookNotificationResponse{
				Status: err.Error(),
			})
		}

		errorWriter.Write(c, w, http.StatusOK, checkoutmodel.WebhookNotificationResponse{
			Status: "[accepted]", // Body containing "[accepted]" is the signal that message has been succesfully processed
		})
	}
}

func parseRequest(r *http.Request) (checkout.CreateCheckoutSessionRequest, string, string, error) {
	basketUID := mux.Vars(r)["basketUID"]
	if basketUID == "" {
		return checkout.CreateCheckoutSessionRequest{}, "", "", myerrors.NewInvalidInputError(fmt.Errorf("missing basketUID:%s", basketUID))
	}

	err := r.ParseForm()
	if err != nil {
		return checkout.CreateCheckoutSessionRequest{}, basketUID, "", myerrors.NewInvalidInputError(err)
	}

	returnURL := r.Form.Get("returnUrl")
	countryCode := r.Form.Get("countryCode")
	currency := r.Form.Get("currency")
	amount, err := strconv.Atoi(r.Form.Get("amount"))
	if err != nil {
		return checkout.CreateCheckoutSessionRequest{}, basketUID, returnURL, myerrors.NewInvalidInputError(fmt.Errorf("invalid amount '%s' (%s)", r.Form.Get("amount"), err))
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

	return checkout.CreateCheckoutSessionRequest{
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
		// MerchantAccount:         "",
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
