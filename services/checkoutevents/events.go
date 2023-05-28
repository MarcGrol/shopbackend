package checkoutevents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myevents"
)

const (
	TopicName             = "checkout"
	checkoutStartedName   = TopicName + ".started"
	checkoutCompletedName = TopicName + ".completed"
	checkoutPaybylinkName = TopicName + ".paybylinkCreated"
)

type CheckoutEventService interface {
	Subscribe(c context.Context) error
	OnCheckoutStarted(c context.Context, topic string, event CheckoutStarted) error
	OnCheckoutCompleted(c context.Context, topic string, event CheckoutCompleted) error
}

func DispatchEvent(c context.Context, reader io.Reader, service CheckoutEventService) error {
	envelope, err := myevents.ParseEventEnvelope(reader)
	if err != nil {
		return myerrors.NewInvalidInputError(err)
	}

	switch envelope.EventTypeName {
	case checkoutStartedName:
		{
			event := CheckoutStarted{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnCheckoutStarted(c, envelope.Topic, event)
		}
	case checkoutCompletedName:
		{
			event := CheckoutCompleted{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnCheckoutCompleted(c, envelope.Topic, event)
		}
	default:
		return myerrors.NewNotImplementedError(fmt.Errorf(envelope.EventTypeName))
	}
}

type CheckoutStarted struct {
	CheckoutUID   string
	ProviderName  string
	AmountInCents int64
	Currency      string
	ShopperUID    string
	MerchantUID   string
}

func (e CheckoutStarted) GetEventTypeName() string {
	return checkoutStartedName
}

func (e CheckoutStarted) GetAggregateName() string {
	return e.CheckoutUID
}

type CheckoutStatus string

const (
	CheckoutStatusUndefined CheckoutStatus = ""
	CheckoutStatusSuccess   CheckoutStatus = "success"
	CheckoutStatusCancelled CheckoutStatus = "cancelled"
	CheckoutStatusPending   CheckoutStatus = "pending"
	CheckoutStatusExpired   CheckoutStatus = "expired"
	CheckoutStatusFailed    CheckoutStatus = "failed"
	CheckoutStatusError     CheckoutStatus = "error"
	CheckoutStatusFraud     CheckoutStatus = "fraud"
	CheckoutStatusOther     CheckoutStatus = "other"
)

type CheckoutCompleted struct {
	CheckoutUID           string
	ProviderName          string
	PaymentMethod         string
	CheckoutStatus        CheckoutStatus
	CheckoutStatusDetails string
}

func (e CheckoutCompleted) GetEventTypeName() string {
	return checkoutCompletedName
}

func (e CheckoutCompleted) GetAggregateName() string {
	return e.CheckoutUID
}

type PayByLinkCreated struct {
	ProviderName  string
	CheckoutUID   string
	AmountInCents int64
	Currency      string
	ShopperUID    string
	MerchantUID   string
}

func (e PayByLinkCreated) GetEventTypeName() string {
	return checkoutPaybylinkName
}

func (e PayByLinkCreated) GetAggregateName() string {
	return e.CheckoutUID
}
