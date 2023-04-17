package mypubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
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
		CreatedAt:     e.nower.Now(),
		Topic:         topic,
		EventTypeName: event.GetEventTypeName(),
		EventPayload:  string(jsonPayload),
		Published:     false,
	}, nil
}

type publisher struct {
	outbox       mystore.Store[EventEnvelope]
	queue        myqueue.TaskQueuer
	enveloper    enveloper
	pubsubClient *pubsub.Client
}

func New(c context.Context, queue myqueue.TaskQueuer, nower mytime.Nower, uuider myuuid.UUIDer) (*publisher, func(), error) {
	store, storeCleanup, err := mystore.New[EventEnvelope](c)
	if err != nil {
		return nil, nil, err
	}

	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	client, err := pubsub.NewClient(c, projectId)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		client.Close()
		storeCleanup()
	}

	return &publisher{
		outbox:       store,
		queue:        queue,
		enveloper:    newEnveloper(nower, uuider),
		pubsubClient: client,
	}, cleanup, nil
}

func (p publisher) RegisterEndpoints(c context.Context, router *mux.Router) {
	router.HandleFunc("/pubsub/{uid}", p.processTriggerPage()).Methods("PUT")
}

func (p publisher) Publish(c context.Context, topic string, event Event) error {
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
		WebhookURLPath: fmt.Sprintf("/pubsub/%s", envelope.UID),
		Payload:        []byte{},
	})
	if err != nil {
		return fmt.Errorf("error queueing publication-trigger %s: %s", envelope.UID, err)
	}

	return nil
}

func (p publisher) processTriggerPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(mylog.New("publisher"))

		eventUID := mux.Vars(r)["uid"]

		err := p.processTrigger(c, eventUID)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{
			Message: "Successfully processed trigger",
		})
	}
}
func (p publisher) processTrigger(c context.Context, uid string) error {
	// fetch all envelopes that are not yet published
	err := p.outbox.RunInTransaction(c, func(c context.Context) error {

		// fetch all envelopes that are not yet published
		envelopes, err := p.outbox.Query(c, "Published", "=", false, "CreatedAt")
		if err != nil {
			return fmt.Errorf("error fetching envelopes: %s", err)
		}
		log.Printf("Found %d unpublished events", len(envelopes))

		for _, envelope := range envelopes {
			log.Printf("Publishing event %s", envelope.UID)

			topic := p.pubsubClient.Topic(envelope.Topic)
			_, err := topic.Publish(c, &pubsub.Message{Data: []byte(envelope.EventPayload)}).Get(c)
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
