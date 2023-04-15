package checkout

import "time"

const (
	TopicName = "checkout"
)

type CheckoutStarted struct {
	CheckoutUID   string
	Timestamp     time.Time
	PaymentMethod string
	AmountInCents int
	shopperUID    string
	merchantUID   string
}

func (CheckoutStarted) GetEventTypeName() string {
	return "checkout.started"
}

type CheckoutCompleted struct {
	CheckoutUID   string
	Success       bool
	Status        string
	PaymentMethod string
}

func (CheckoutCompleted) GetEventTypeName() string {
	return "checkout.completed"
}
