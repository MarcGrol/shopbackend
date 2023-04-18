package mypublisher

import (
	"encoding/json"

	"github.com/MarcGrol/shopbackend/services/checkout/checkoutevents"

	"github.com/MarcGrol/shopbackend/lib/myevents"
	"github.com/MarcGrol/shopbackend/lib/mytime"
)

func CreatePubsubMessage(event checkoutevents.CheckoutCompleted) string {
	eventBytes, _ := json.Marshal(event)
	envelope := myevents.EventEnvelope{
		UID:           "123",
		CreatedAt:     mytime.ExampleTime,
		Topic:         "checkout",
		AggregateUID:  "111",
		EventTypeName: "checkout.completed",
		EventPayload:  string(eventBytes),
	}
	envelopeBytes, _ := json.Marshal(envelope)

	req := myevents.PushRequest{
		Message: myevents.PushMessage{
			Data: envelopeBytes,
		},
		Subscription: "checkout",
	}

	reqBytes, _ := json.Marshal(req)

	return string(reqBytes)
}
