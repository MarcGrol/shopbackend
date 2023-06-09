package checkoutapi

import (
	"time"

	"github.com/MarcGrol/shopbackend/services/checkoutevents"
)

func NewCheckoutContext() CheckoutContext {
	return CheckoutContext{
		CheckoutStatus: checkoutevents.CheckoutStatusUndefined,
	}
}

type CheckoutContext struct {
	BasketUID             string
	CreatedAt             time.Time
	LastModified          *time.Time
	OriginalReturnURL     string
	ID                    string
	SessionData           string `datastore:",noindex"`
	PayByLink             bool
	Status                string
	PaymentProvider       string
	PaymentMethod         string
	CheckoutStatus        checkoutevents.CheckoutStatus
	CheckoutStatusDetails string
}
