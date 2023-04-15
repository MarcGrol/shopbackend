package mypubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
)

type enveloper struct {
	nower  mytime.Nower
	uuider myuuid.UUIDer
}

func newEnveloper(nower mytime.Nower, uuider myuuid.UUIDer) enveloper {
	return enveloper{
		nower:  nower,
		uuider: uuider,
	}
}

func (e enveloper) do(topic string, event Event) (EventEnvelope, error) {
	jsonPayload, err := json.Marshal(event)
	if err != nil {
		return EventEnvelope{}, err
	}
	return EventEnvelope{
		UID:           e.uuider.Create(),
		Timestamp:     e.nower.Now(),
		Topic:         topic,
		EventTypeName: event.GetEventTypeName(),
		EventPayload:  string(jsonPayload),
		Published:     false,
	}, nil
}

type eventStore struct {
	store     mystore.Store[EventEnvelope]
	enveloper enveloper
}

func New(c context.Context, nower mytime.Nower, uuider myuuid.UUIDer) (Publisher, func(), error) {
	store, storeCleanup, err := mystore.New[EventEnvelope](c)
	if err != nil {
		return nil, nil, err
	}
	return &eventStore{
		store:     store,
		enveloper: newEnveloper(nower, uuider),
	}, storeCleanup, nil
}

func (es eventStore) Publish(c context.Context, topic string, event Event) error {
	envelope, err := es.enveloper.do(topic, event)
	if err != nil {
		return fmt.Errorf("error creating envelope: %s", err)
	}
	err = es.store.Put(c, envelope.UID, envelope)
	if err != nil {
		return fmt.Errorf("error storing envelope: %s", err)
	}

	// TODO trigger event forwarder

	return nil
}
