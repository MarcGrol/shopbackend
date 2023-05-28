package checkoutapi

import (
	"time"

	"github.com/MarcGrol/shopbackend/services/checkoutevents"
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
	PayByLink           bool
	Status              string
	PaymentMethod       string
	WebhookEventName    string
	WebhookEventSuccess bool
	CheckoutStatus      checkoutevents.CheckoutStatus
	CheckoutDetails     string
}
