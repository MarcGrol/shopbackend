package checkoutstripe

import "fmt"

type CheckoutPageInfo struct {
	Environment     string
	MerchantAccount string
	ClientKey       string
	BasketUID       string
	Amount          Amount
	CountryCode     string
	ShopperLocale   string
	ShopperEmail    string
	ID              string
	SessionData     string
}

type Amount struct {
	Currency string
	Value    int64
}

func (a Amount) String() string {
	return fmt.Sprintf("%s %.2f", a.Currency, float32(a.Value/100.00))
}
