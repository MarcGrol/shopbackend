package checkoutadyen

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

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
func NewWebService(cfg Config, payer Payer, checkoutStore mystore.Store[CheckoutContext], vault myvault.VaultReader, nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) (*webService, error) {
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

		log.Printf("sessionRequest:%+v", sessionRequest)
		log.Printf("sessionRequest:%+v", *sessionRequest.LineItems)

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

	/*
			  <input type="hidden" name="product.count" value="0"/>

		            <input type="hidden" name="product.0.name" value="product_running_socks"/>
		            <input type="hidden" name="product.0.description" value="Running socks"/>
		            <input type="hidden" name="product.0.itemPrice" value="1000"/>
		            <input type="hidden" name="product.0.currency" value="EUR"/>
		            <input type="hidden" name="product.0.quantity" value="3"/>
		            <input type="hidden" name="product.0.totalPrice" value="3000"/>

		            <input type="hidden" name="product.1.name" value="product_tennis_balls"/>
		            <input type="hidden" name="product.1.description" value="Tennis balls"/>
		            <input type="hidden" name="product.1.itemPrice" value="1000"/>
		            <input type="hidden" name="product.1.currency" value="EUR"/>
		            <input type="hidden" name="product.1.quantity" value="6"/>
		            <input type="hidden" name="product.1.totalPrice" value="6000"/>

	*/
	productCount, err := strconv.Atoi(r.Form.Get("product.count"))
	if err != nil {
		return checkout.CreateCheckoutSessionRequest{}, basketUID, returnURL, myerrors.NewInvalidInputError(fmt.Errorf("invalid product count '%s' (%s)", r.Form.Get("product.count"), err))
	}

	products := []checkout.LineItem{}
	for i := 0; i < productCount; i++ {
		p := checkout.LineItem{
			Id:          r.Form.Get(fmt.Sprintf("product.%d.name", i)),
			Description: r.Form.Get(fmt.Sprintf("product.%d.description", i)),
			AmountIncludingTax: func() int64 {
				price, err := strconv.Atoi(r.Form.Get(fmt.Sprintf("product.%d.itemPrice", i)))
				if err != nil {
					return 0
				}
				return int64(price)
			}(),
			Quantity: func() int64 {
				quantity, err := strconv.Atoi(r.Form.Get(fmt.Sprintf("product.%d.quantity", i)))
				if err != nil {
					return 0
				}
				return int64(quantity)
			}(),
		}
		products = append(products, p)
	}

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
		LineItems: &products,
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
