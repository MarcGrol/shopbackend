package checkout

import (
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
)

type CheckoutContext struct {
	BasketUID         string
	OriginalReturnURL string
	SessionRequest    checkout.CreateCheckoutSessionRequest
	SessionResponse   checkout.CreateCheckoutSessionResponse
	PaymentMethods    checkout.PaymentMethodsResponse
	Status            string
}

type CheckoutPageInfo struct {
	BasketUID              string
	PaymentMethodsResponse checkout.PaymentMethodsResponse
	ClientKey              string
	MerchantAccount        string
	CountryCode            string
	ShopperLocale          string
	Environment            string
	ShopperEmail           string
	Session                checkout.CreateCheckoutSessionResponse
}
