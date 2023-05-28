package shopevents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myevents"
)

const (
	TopicName              = "basket"
	basketCreateName       = TopicName + ".created"
	basketPaymentCompleted = TopicName + ".payment.completed"
)

type BasketEventService interface {
	Subscribe(c context.Context) error
	OnBasketCreated(c context.Context, topic string, event BasketCreated) error
	OnBasketPaymentCompleted(c context.Context, topic string, event BasketPaymentCompleted) error
}

func DispatchEvent(c context.Context, reader io.Reader, service BasketEventService) error {
	envelope, err := myevents.ParseEventEnvelope(reader)
	if err != nil {
		return myerrors.NewInvalidInputError(err)
	}

	switch envelope.EventTypeName {
	case basketCreateName:
		{
			event := BasketCreated{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnBasketCreated(c, envelope.Topic, event)
		}
	case basketPaymentCompleted:
		{
			event := BasketPaymentCompleted{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnBasketPaymentCompleted(c, envelope.Topic, event)
		}
	default:
		return myerrors.NewNotImplementedError(fmt.Errorf(envelope.EventTypeName))
	}
}

type BasketCreated struct {
	BasketUID string
}

func (e BasketCreated) GetEventTypeName() string {
	return basketCreateName
}

func (e BasketCreated) GetAggregateName() string {
	return e.BasketUID
}

type BasketPaymentCompleted struct {
	BasketUID string
}

func (e BasketPaymentCompleted) GetEventTypeName() string {
	return basketPaymentCompleted
}

func (e BasketPaymentCompleted) GetAggregateName() string {
	return e.BasketUID
}
