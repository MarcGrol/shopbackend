package mypublisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myevents"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
)

type transactionalPublisher struct {
	outbox    mystore.Store[myevents.EventEnvelope]
	queue     myqueue.TaskQueuer
	enveloper enveloper
	pubsub    mypubsub.PubSub
}

func New(c context.Context, pubsub mypubsub.PubSub, queue myqueue.TaskQueuer, nower mytime.Nower) (*transactionalPublisher, func(), error) {
	store, storeCleanup, err := mystore.New[myevents.EventEnvelope](c)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		storeCleanup()
	}

	return &transactionalPublisher{
		outbox:    store,
		queue:     queue,
		enveloper: newEnveloper(nower),
		pubsub:    pubsub,
	}, cleanup, nil
}

func (p *transactionalPublisher) RegisterEndpoints(c context.Context, router *mux.Router) {
	router.HandleFunc("/pubsub/{topic}/{uid}", p.processTriggerPage()).Methods("PUT")
}

func (p *transactionalPublisher) CreateTopic(c context.Context, topicName string) error {
	return p.pubsub.CreateTopic(c, topicName)
}

func (p *transactionalPublisher) Publish(c context.Context, topic string, event myevents.Event) error {
	envelope, err := p.enveloper.do(topic, event)
	if err != nil {
		return fmt.Errorf("error creating envelope: %s", err)
	}
	err = p.outbox.Put(c, envelope.UID, envelope)
	if err != nil {
		return fmt.Errorf("error storing envelope: %s", err)
	}

	err = p.queue.Enqueue(c, myqueue.Task{
		UID:            envelope.UID,
		WebhookURLPath: fmt.Sprintf("/pubsub/%s/%s", envelope.Topic, envelope.UID),
		Payload:        []byte{},
	})
	if err != nil {
		return fmt.Errorf("error queueing publication-trigger %s: %s", envelope.UID, err)
	}

	log.Printf("Enqueued event %s.%s on topic %s", envelope.EventTypeName, envelope.AggregateUID, envelope.Topic)

	return nil
}

func (p *transactionalPublisher) processTriggerPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(mylog.New("transactionalPublisher"))

		topicName := mux.Vars(r)["topic"]
		eventUID := mux.Vars(r)["uid"]

		err := p.processTrigger(c, topicName, eventUID)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{
			Message: "Successfully processed trigger",
		})
	}
}

func (p *transactionalPublisher) processTrigger(c context.Context, topicName string, uid string) error {
	// fetch all envelopes that are not yet published
	err := p.outbox.RunInTransaction(c, func(c context.Context) error {

		// fetch all envelopes that are not yet published
		envelopes, err := p.outbox.Query(c, []mystore.Filter{{Field: "Published", Compare: "=", Value: false}}, "CreatedAt")
		if err != nil {
			return fmt.Errorf("error fetching envelopes: %s", err)
		}
		log.Printf("Found %d unpublished events", len(envelopes))

		for _, envelope := range envelopes {
			jsonBytes, err := json.Marshal(envelope)
			if err != nil {
				return fmt.Errorf("error serializing event: %s", err)
			}

			err = p.pubsub.Publish(c, envelope.Topic, string(jsonBytes))
			if err != nil {
				return fmt.Errorf("error publishing event: %s", err)
			}

			// mark as published
			envelope.Published = true
			err = p.outbox.Put(c, envelope.UID, envelope)
			if err != nil {
				return fmt.Errorf("error store envelope: %s", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
