package checkoutevents

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

func (e CheckoutStarted) GetEventTypeName() string {
	return "checkout.started"
}

func (e CheckoutStarted) GetAggregateName() string {
	return e.CheckoutUID
}

type CheckoutCompleted struct {
	CheckoutUID   string
	PaymentMethod string
	Status        string
	Success       bool
}

func (e CheckoutCompleted) GetEventTypeName() string {
	return "checkout.completed"
}

func (e CheckoutCompleted) GetAggregateName() string {
	return e.CheckoutUID
}
