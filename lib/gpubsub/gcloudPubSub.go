package gpubsub

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
)

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		New = newGcloudPubSub
	}
}

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
		}, func() {
			client.Close()
		}, nil
}

func (ps *gcloudPubSub) Subscribe(c context.Context, topicName string, urlToPostTo string) error {
	err := ps.CreateTopic(c, topicName)
	if err != nil {
		return err
	}
	topic := ps.client.Topic(topicName)
	_, err = ps.client.CreateSubscription(c, topicName, pubsub.SubscriptionConfig{
		Topic: topic,
		PushConfig: pubsub.PushConfig{
			Endpoint: urlToPostTo,
		},
		AckDeadline:                   time.Second * 10,
		RetentionDuration:             time.Hour * 24,
		ExpirationPolicy:              time.Duration(0),
		TopicMessageRetentionDuration: time.Hour * 24,
	})
	if err != nil {
		return fmt.Errorf("error subscribing to topic %s: %s", topicName, err)
	}

	return nil
}

func (ps *gcloudPubSub) CreateTopic(c context.Context, topicName string) error {
	topic := ps.client.Topic(topicName)
	exists, err := topic.Exists(c)
	if err != nil {
		return fmt.Errorf("error checking if topic %s exists: %s", topicName, err)
	}
	if exists {
		return nil
	}

	_, err = ps.client.CreateTopic(c, topicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", topicName, err)
	}

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
		return fmt.Errorf("error publishing event: %s", err)
	}
	return nil
}
