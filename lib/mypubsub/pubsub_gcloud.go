package mypubsub

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
)

type gcloudPubSub struct {
	client *pubsub.Client
	topics map[string]*pubsub.Topic
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		New = newGcloudPubSub
	}
}

func newGcloudPubSub(c context.Context) (PubSub, func(), error) {
	client, err := pubsub.NewClient(c, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return nil, func() {}, err
	}
	return &gcloudPubSub{
			client: client,
			topics: map[string]*pubsub.Topic{},
		}, func() {
			client.Close()
		}, nil
}

func (ps *gcloudPubSub) Subscribe(c context.Context, topicName string, urlToPostTo string) error {
	err := ps.CreateTopic(c, topicName)
	if err != nil {
		return err
	}

	subscription := ps.client.Subscription(topicName)
	exists, err := subscription.Exists(c)
	if err != nil {
		return fmt.Errorf("error checking if subscription %s (%s) exists: %s", topicName, urlToPostTo, err)
	}

	if exists {
		log.Printf("*** Subsription %s (%s) already exists", topicName, urlToPostTo)
		return nil
	}

	topic := ps.client.Topic(topicName)
	_, err = ps.client.CreateSubscription(c, topicName, pubsub.SubscriptionConfig{
		Topic: topic,
		PushConfig: pubsub.PushConfig{
			Endpoint: urlToPostTo,
		},
	})
	if err != nil {
		return fmt.Errorf("error subscribing to topic %s (%s): %s", topicName, urlToPostTo, err)
	}

	log.Printf("*** Subscribed to topic %s (%s)", topic.String(), urlToPostTo)

	return nil
}

func (ps *gcloudPubSub) CreateTopic(c context.Context, topicName string) error {
	topic := ps.client.Topic(topicName)
	exists, err := topic.Exists(c)
	if err != nil {
		return fmt.Errorf("error checking if topic %s exists: %s", topicName, err)
	}

	if exists {
		log.Printf("*** Topic %s already exists", topicName)
		return nil
	}

	_, err = ps.client.CreateTopic(c, topicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", topicName, err)
	}
	ps.topics[topicName] = topic

	log.Printf("*** Created topic %s", topicName)

	return nil
}

func (ps *gcloudPubSub) Publish(c context.Context, topicName string, data string) error {
	topic, found := ps.topics[topicName]
	if !found {
		topic = ps.client.Topic(topicName)
		ps.topics[topicName] = topic
	}

	_, err := topic.Publish(c, &pubsub.Message{Data: []byte(data)}).Get(c)
	if err != nil {
		return fmt.Errorf("error publishing event on topic %s: %s", topicName, err)
	}

	return nil
}
