package checkout

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/adyen/adyen-go-api-library/v6/src/adyen"
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"github.com/adyen/adyen-go-api-library/v6/src/common"

	"github.com/MarcGrol/shopbackend/myerrors"
	"github.com/MarcGrol/shopbackend/myhttp"
)

const (
	apiKeyVarname    = "ADYEN_API_KEY"
	clientKeyVarname = "ADYEN_CLIENT_KEY"
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
	clientKey     string
	apiClient     *adyen.APIClient
	checkoutStore CheckoutStore
}

type Starter interface {
	PrepareForCheckoutPage(basketUID string, req checkout.CreateCheckoutSessionRequest) (string, error)
}

func NewService(checkoutStore CheckoutStore) (*service, error) {
	apiKey := os.Getenv(apiKeyVarname)
	if apiKey == "" {
		return nil, myerrors.NewInvalidInputError(fmt.Errorf("Missing env-var %s", apiKeyVarname))
	}

	clientKey := os.Getenv(clientKeyVarname)
	if clientKey == "" {
		return nil, myerrors.NewInvalidInputError(fmt.Errorf("Missing env-var %s", clientKeyVarname))
	}

	return &service{
		clientKey: clientKey,
		apiClient: adyen.NewClient(&common.Config{
			ApiKey:      apiKey,
			Environment: common.TestEnv,
			//Debug:       true,
		}),
		checkoutStore: checkoutStore,
	}, nil
}

func (s service) RegisterEndpoints(c context.Context, router *mux.Router) {
	router.HandleFunc("/checkout/{basketUID}", s.startCheckoutPage()).Methods("POST")
	router.HandleFunc("/checkout/{basketUID}", s.finalizeCheckoutPage()).Methods("GET")
	router.HandleFunc("/checkout/{basketUID}/status/{status}", s.statusCallback()).Methods("GET")

	// TODO listen for incoming webhooks to update the basket and convert it into an order

}

func (s service) startCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		// parse request and convert into CreateCheckoutSessionRequest
		sessionRequest, basketUID, returnURL, err := parseRequest(r)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInvalidInputError(fmt.Errorf("Error parsing request: %s", err)))
			return
		}
		{
			data, _ := json.MarshalIndent(sessionRequest, "", "\t")
			log.Printf("startCheckoutPage: %+v", string(data))
		}

		// store checkout-context
		checkoutSessionResp, _, err := s.apiClient.Checkout.Sessions(sessionRequest)
		if err != nil {
			myhttp.WriteError(w, 2, fmt.Errorf("Error creating payment session for checkout %s: %s", basketUID, err))
			return
		}

		paymentMethodsRequest := checkoutToPaymentMethodsRequest(sessionRequest)
		paymentMethodsResp, _, err := s.apiClient.Checkout.PaymentMethods(&paymentMethodsRequest)
		if err != nil {
			myhttp.WriteError(w, 3, fmt.Errorf("Error fetching payment methods for checkoutContext %s: %s", basketUID, err))
			return
		}

		checkoutContext := CheckoutContext{
			BasketUID:         basketUID,
			OriginalReturnURL: returnURL,
			SessionRequest:    *sessionRequest,
			SessionResponse:   checkoutSessionResp,
			PaymentMethods:    paymentMethodsResp,
			Status:            "",
		}

		err = s.checkoutStore.Put(c, basketUID, checkoutContext)
		if err != nil {
			myhttp.WriteError(w, 4, myerrors.NewInternalError(fmt.Errorf("Error storing checkout: %s", err)))
			return
		}

		pageInfo := CheckoutPageInfo{
			BasketUID:              basketUID,
			PaymentMethodsResponse: checkoutContext.PaymentMethods,
			ClientKey:              s.clientKey,
			MerchantAccount:        checkoutContext.SessionRequest.MerchantAccount,
			CountryCode:            checkoutContext.SessionRequest.CountryCode,
			ShopperLocale:          checkoutContext.SessionRequest.ShopperLocale,
			Environment:            string(common.TestEnv),
			ShopperEmail:           checkoutContext.SessionRequest.ShopperEmail,
			Session:                checkoutContext.SessionResponse,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = checkoutPageTemplate.Execute(w, pageInfo)
		if err != nil {
			myhttp.WriteError(w, 5, myerrors.NewInternalError(fmt.Errorf("Error executimng template: %s", err)))
			return
		}
	}
}

func (s service) finalizeCheckoutPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		basketUID := mux.Vars(r)["basketUID"]

		checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
		if err != nil {
			myhttp.WriteError(w, 10, myerrors.NewInternalError(err))
			return
		}
		if !found {
			myhttp.WriteError(w, 11, myerrors.NewNotFoundError(fmt.Errorf("Checkout with uid %s not found", basketUID)))
			return
		}

		log.Printf("Resume checkout for basket %s", basketUID)

		pageInfo := CheckoutPageInfo{
			BasketUID:              basketUID,
			PaymentMethodsResponse: checkoutContext.PaymentMethods,
			ClientKey:              s.clientKey,
			MerchantAccount:        checkoutContext.SessionRequest.MerchantAccount,
			CountryCode:            checkoutContext.SessionRequest.CountryCode,
			ShopperLocale:          checkoutContext.SessionRequest.ShopperLocale,
			Environment:            string(common.TestEnv),
			ShopperEmail:           checkoutContext.SessionRequest.ShopperEmail,
			Session:                checkoutContext.SessionResponse,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = checkoutPageTemplate.Execute(w, pageInfo)
		if err != nil {
			myhttp.WriteError(w, 12, myerrors.NewInternalError(fmt.Errorf("Error executimng template: %s", err)))
			return
		}
	}
}

func (s service) statusCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		basketUID := mux.Vars(r)["basketUID"]
		status := mux.Vars(r)["status"]

		log.Printf("Checkout completed for %s -> %s", basketUID, status)

		checkoutContext, found, err := s.checkoutStore.Get(c, basketUID)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}
		if !found {
			myhttp.WriteError(w, 1, myerrors.NewNotFoundError(fmt.Errorf("Checkout with uid %s not found", basketUID)))
			return
		}

		checkoutContext.Status = status
		err = s.checkoutStore.Put(c, basketUID, checkoutContext)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}

		u, err := url.Parse(checkoutContext.OriginalReturnURL)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInvalidInputError(err))
		}
		params := u.Query()
		params.Set("status", status)
		u.RawQuery = params.Encode()
		adjustedReturnURL := u.String()
		log.Printf("Redirecting to %s", adjustedReturnURL)

		http.Redirect(w, r, adjustedReturnURL, http.StatusSeeOther)
	}
}

func parseRequest(r *http.Request) (*checkout.CreateCheckoutSessionRequest, string, string, error) {
	basketUID := mux.Vars(r)["basketUID"]
	if basketUID == "" {
		return nil, "", "", myerrors.NewInvalidInputError(fmt.Errorf("basketUID:%s, err"))
	}

	err := r.ParseForm()
	if err != nil {
		return nil, basketUID, "", myerrors.NewInvalidInputError(err)
	}

	returnURL := r.Form.Get("returnUrl")
	amount, err := strconv.Atoi(r.Form.Get("amount"))
	if err != nil {
		return nil, basketUID, returnURL, myerrors.NewInvalidInputError(fmt.Errorf("amount:%s, err"))
	}
	currency := r.Form.Get("currency")

	addressCity := r.Form.Get("address.city")
	addressCountry := r.Form.Get("address.country")
	addressHouseNumber := r.Form.Get("address.houseNumber")
	addressPostalCode := r.Form.Get("address.postalCode")
	addressStateOrProvince := r.Form.Get("address.state")
	addressStreet := r.Form.Get("address.street")

	companyHomepage := r.Form.Get("company.homepage")
	companyName := r.Form.Get("company.name")
	//shopName := r.Form.Get("shop.name") // Thios one causes problems

	countryCode := r.Form.Get("countryCode")
	merchantAccount := r.Form.Get("merchantAccount")
	shopperEmail := r.Form.Get("shopper.email")

	shopperDateOfBirth := func() *time.Time {
		dob := r.Form.Get("shopper.dateOfBirth")
		if dob == "" {
			return nil
		}
		t, err := time.Parse(time.DateOnly, r.Form.Get("shopper.dateOfBirth"))
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
		ReturnUrl:          fmt.Sprintf("%s/checkout/%s",  myhttp.HostnameWithScheme(r), basketUID),
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

func checkoutToPaymentMethodsRequest(checkoutReq *checkout.CreateCheckoutSessionRequest) checkout.PaymentMethodsRequest {
	return checkout.PaymentMethodsRequest{
		CountryCode:   checkoutReq.CountryCode,
		ShopperLocale: checkoutReq.ShopperLocale,
		Channel:       "Web",
		Amount: &checkout.Amount{
			Currency: checkoutReq.Amount.Currency,
			Value:    checkoutReq.Amount.Value,
		},
		MerchantAccount: checkoutReq.MerchantAccount,
	}
}
