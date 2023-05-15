package checkoutapi

import (
	"time"
)

func NewCheckoutContext() CheckoutContext {
	return CheckoutContext{
		WebhookEventName:    "unknown",
		WebhookEventSuccess: false,
	}
}

type CheckoutContext struct {
	BasketUID           string
	CreatedAt           time.Time
	LastModified        *time.Time
	OriginalReturnURL   string
	ID                  string
	SessionData         string `datastore:",noindex"`
	Status              string
	PaymentMethod       string
	WebhookEventName    string
	WebhookEventSuccess bool
}
