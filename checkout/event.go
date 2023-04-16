package checkout

const (
	TopicName = "checkout"
)

type CheckoutStarted struct {
	CheckoutUID   string
	AmountInCents int64
	Currency      string
	ShopperUID    string
	MerchantUID   string
}

func (CheckoutStarted) GetEventTypeName() string {
	return "checkout.started"
}

type CheckoutCompleted struct {
	CheckoutUID   string
	PaymentMethod string
	Status        string
	Success       bool
}

func (CheckoutCompleted) GetEventTypeName() string {
	return "checkout.completed"
}
