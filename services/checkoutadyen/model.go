package checkoutadyen

import (
	"fmt"
	"time"

	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
)

func NewCheckoutContext() CheckoutContext {
	return CheckoutContext{
		WebhookStatus:  "unknown",
		WebhookSuccess: "unknown",
	}
}

type CheckoutContext struct {
	BasketUID         string
	CreatedAt         time.Time
	LastModified      *time.Time
	OriginalReturnURL string
	ID                string
	SessionData       string `datastore:",noindex"`
	Status            string
	PaymentMethod     string
	WebhookStatus     string
	WebhookSuccess    string
}

type CheckoutPageInfo struct {
	Environment            string
	MerchantAccount        string
	ClientKey              string
	BasketUID              string
	Amount                 Amount
	CountryCode            string
	ShopperLocale          string
	ShopperEmail           string
	PaymentMethodsResponse checkout.PaymentMethodsResponse
	ID                     string
	SessionData            string
}

func (ci CheckoutPageInfo) AmountFormatted() string {
	return fmt.Sprintf("%.2f %s", float32(ci.Amount.Value)/100.0, ci.Amount.Currency)
}

type WebhookNotification struct {
	Live              string             `json:"live"`
	NotificationItems []NotificationItem `json:"notificationItems"`
}

type WebhookNotificationResponse struct {
	Status string `json:"status"`
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
	Currency string
	Value    int64
}

func (a Amount) String() string {
	return fmt.Sprintf("%s %.2f", a.Currency, float32(a.Value/100.00))
}

type AuthTokenUpdateEvent struct {
	AccessToken string
}
