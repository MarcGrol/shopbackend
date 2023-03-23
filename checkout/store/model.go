package store

import (
	"fmt"
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"time"
)

type CheckoutContext struct {
	BasketUID         string
	OriginalReturnURL string
	SessionRequest    checkout.CreateCheckoutSessionRequest
	SessionResponse   checkout.CreateCheckoutSessionResponse
	PaymentMethods    checkout.PaymentMethodsResponse
	Status            string
	WebhookStatus     string
	PaymentMethod     string
	WebhookSuccess    string
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

func (ci CheckoutPageInfo) Amount() string {
	return fmt.Sprintf("%.2f %s", float32(ci.Session.Amount.Value)/100.0, ci.Session.Amount.Currency)
}

type WebhookNotification struct {
	Live              string             `json:"live"`
	NotificationItems []NotificationItem `json:"notificationItems"`
}

type NotificationItem struct {
	NotificationRequestItem NotificationRequestItem `json:"NotificationRequestItem"`
}

type NotificationRequestItem struct {
	AdditionalData      AdditionalData `json:"additionalData"`
	Amount              Amount         `json:"amount"`
	EventCode           string         `json:"eventCode"`
	EventDate           time.Time      `json:"eventDate"`
	MerchantAccountCode string         `json:"merchantAccountCode"`
	MerchantReference   string         `json:"merchantReference"`
	Operations          []string       `json:"operations"`
	PaymentMethod       string         `json:"paymentMethod"`
	PspReference        string         `json:"pspReference"`
	Reason              string         `json:"reason"`
	Success             string         `json:"success"`
}

type AdditionalData struct {
	CheckoutSessionId string `json:"checkoutSessionId"`
}

type Amount struct {
	Currency string `json:"currency"`
	Value    int    `json:"value"`
}
