package checkoutstripe

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
)

type webService struct {
	logger  mylog.Logger
	service *service
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewWebService() (*webService, error) {
	logger := mylog.New("checkoutstripe")
	s, err := newService(logger)
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

		errorWriter.Write(c, w, 200, resp)
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
