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
	TopicName           = "basket"
	basketFinalizedName = TopicName + ".finalized"
)

type BasketEventService interface {
	Subscribe(c context.Context) error
	OnBasketFinalized(c context.Context, topic string, event BasketFinalized) error
}

func DispatchEvent(c context.Context, reader io.Reader, service BasketEventService) error {
	envelope, err := myevents.ParseEventEnvelope(reader)
	if err != nil {
		return myerrors.NewInvalidInputError(err)
	}

	switch envelope.EventTypeName {
	case basketFinalizedName:
		{
			event := BasketFinalized{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnBasketFinalized(c, envelope.Topic, event)
		}
	default:
		return myerrors.NewNotImplementedError(fmt.Errorf(envelope.EventTypeName))
	}
}

type BasketFinalized struct {
	BasketUID string
}

func (e BasketFinalized) GetEventTypeName() string {
	return basketFinalizedName
}

func (e BasketFinalized) GetAggregateName() string {
	return e.BasketUID
}
