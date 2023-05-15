package checkoutadyen

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
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
func NewWebService(cfg Config, payer Payer, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader, nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) (*webService, error) {
	logger := mylog.New("checkoutadyen")
	s, err := newCommandService(cfg, payer, checkoutStore, vault, nower, logger, subscriber, publisher)
	if err != nil {
		return nil, err
	}

	return &webService{
		logger:  logger,
		service: s,
	}, nil
}

func (s *webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	// TODO: subscribe to receive access-token updates

	// Endpoints that compose the user-interface
	router.HandleFunc("/checkout/{basketUID}", s.startCheckoutPage()).Methods("POST")
	router.HandleFunc("/checkout/{basketUID}", s.resumeCheckoutPage()).Methods("GET")

	// Adyen will redirect to this endpoint after checkout has finalized
	router.HandleFunc("/checkout/{basketUID}/status/{status}", s.finalizeCheckoutPage()).Methods("GET")

	// Final notification called by Adyen at a later time
	router.HandleFunc("/checkout/webhook/event", s.webhookNotification()).Methods("POST")

	err := s.service.CreateTopics(c)
	if err != nil {
		return err
	}

	// Listen for token refresh
	router.HandleFunc("/api/checkout/event", s.handleEventEnvelope()).Methods("POST")

	err = s.service.Subscribe(c)
	if err != nil {
		return err
	}

	return nil
}

// startCheckoutPage starts a checkout session on the Adyen platform
func (s *webService) startCheckoutPage() http.HandlerFunc {
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
func (s *webService) resumeCheckoutPage() http.HandlerFunc {
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
func (s *webService) webhookNotification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		username, password, _ := r.BasicAuth()

		event := WebhookNotification{}
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			errorWriter.WriteError(c, w, 1, fmt.Errorf("error parsing webhook notification event:%s", err))
			return
		}

		err = s.service.webhookNotification(c, username, password, event)
		if err != nil {
			errorWriter.Write(c, w, http.StatusOK, WebhookNotificationResponse{
				Status: err.Error(),
			})
			return
		}

		errorWriter.Write(c, w, http.StatusOK, WebhookNotificationResponse{
			Status: "[accepted]", // Body containing "[accepted]" is the signal that message has been succesfully processed
		})
	}
}

func (s *webService) handleEventEnvelope() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		err := oauthevents.DispatchEvent(c, r.Body, s.service)
		if err != nil {
			errorWriter.WriteError(c, w, 4, err)
			return
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{
			Message: "Successfully processed event",
		})
	}
}

func parseRequest(r *http.Request) (checkout.CreateCheckoutSessionRequest, string, string, error) {
	basketUID := mux.Vars(r)["basketUID"]
	if basketUID == "" {
		return checkout.CreateCheckoutSessionRequest{}, "", "", myerrors.NewInvalidInputError(fmt.Errorf("missing basketUID:%s", basketUID))
	}

	co, err := checkoutapi.NewFromRequest(r)
	if err != nil {
		return checkout.CreateCheckoutSessionRequest{}, "", "", myerrors.NewInvalidInputError(fmt.Errorf("error parsing form: %s", err))
	}

	return checkout.CreateCheckoutSessionRequest{
		//AccountInfo:           nil,
		//AdditionalAmount:      nil,
		//AdditionalData:        nil,
		AllowedPaymentMethods: []string{"ideal", "scheme"},
		Amount: checkout.Amount{
			Currency: co.TotalAmount.Currency,
			Value:    int64(co.TotalAmount.Value),
		},
		//ApplicationInfo:    nil,
		//AuthenticationData: nil,
		BillingAddress: &checkout.Address{
			City:              co.Shopper.Address.City,
			Country:           co.Shopper.Address.Country,
			HouseNumberOrName: co.Shopper.Address.AddressHouseNumber,
			PostalCode:        co.Shopper.Address.PostalCode,
			StateOrProvince:   co.Shopper.Address.State,
			Street:            co.Shopper.Address.Street,
		},
		//BlockedPaymentMethods: []string{},
		//CaptureDelayHours:     0,
		Channel: "Web",
		Company: &checkout.Company{
			Homepage:           co.Company.Homepage,
			Name:               co.Company.Name,
			RegistrationNumber: "",
			RegistryLocation:   "",
			TaxId:              "",
			Type:               "",
		},
		CountryCode: co.Company.CountryCode,
		//DateOfBirth: nil
		//DeliverAt:   nil,
		DeliveryAddress: &checkout.Address{
			City:              co.Shopper.Address.City,
			Country:           co.Shopper.Address.Country,
			HouseNumberOrName: co.Shopper.Address.AddressHouseNumber,
			PostalCode:        co.Shopper.Address.PostalCode,
			StateOrProvince:   co.Shopper.Address.State,
			Street:            co.Shopper.Address.Street,
		},
		//EnableOneClick:           false,
		//EnablePayOut:             false,
		//EnableRecurring:          false,
		//ExpiresAt: &expiresAt,
		LineItems: func() *[]checkout.LineItem {
			products := []checkout.LineItem{}
			for _, p := range co.Products {
				products = append(products, checkout.LineItem{
					Id:                 p.Name,
					Description:        p.Description,
					AmountIncludingTax: int64(p.ItemPrice),
					Quantity:           int64(p.Quantity),
				})
			}
			return &products
		}(),
		//Mandate:                  nil,
		//Mcc:                      "",
		//MerchantAccount:         "",
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
		ShopperEmail:       co.Shopper.ContactInfo.Email,
		ShopperIP:          "",
		ShopperInteraction: "",
		ShopperLocale:      co.Shopper.Locale,
		ShopperName: &checkout.Name{
			FirstName: co.Shopper.FirstName,
			LastName:  co.Shopper.LastName,
		},
		ShopperReference: co.Shopper.UID,
		//ShopperStatement:          "",
		//SocialSecurityNumber:      "",
		//SplitCardFundingSources:   false,
		//Splits:                    nil,
		//Store: shopName,
		//StorePaymentMethod:        false,
		TelephoneNumber: co.Shopper.ContactInfo.PhoneNumber,
		//ThreeDSAuthenticationOnly: false,
		TrustedShopper: true,
	}, basketUID, co.ReturnURL, nil
}
