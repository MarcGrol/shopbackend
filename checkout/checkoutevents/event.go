package checkoutevents

const (
	TopicName             = "checkout"
	CheckoutStartedName   = TopicName + ".started"
	CheckoutCompletedName = TopicName + ".completed"
)

type CheckoutStarted struct {
	CheckoutUID   string
	AmountInCents int64
	Currency      string
	ShopperUID    string
	MerchantUID   string
}

func (e CheckoutStarted) GetEventTypeName() string {
	return CheckoutStartedName
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
	return CheckoutCompletedName
}

func (e CheckoutCompleted) GetAggregateName() string {
	return e.CheckoutUID
}
