package mypubsub

import (
	"context"
	"os"
)

type fakePubSub struct {
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		New = newFakePubSub
	}
}

func newFakePubSub(c context.Context) (PubSub, func(), error) {
	return &fakePubSub{}, func() {
	}, nil
}

func (q *fakePubSub) Subscribe(c context.Context, topic string, urlToPostTo string) error {
	return nil
}

func (q *fakePubSub) CreateTopic(c context.Context, topic string) error {
	return nil
}

func (q *fakePubSub) Publish(c context.Context, topic string, data string) error {
	return nil
}
