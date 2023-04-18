package mypubsub

import "context"

//go:generate mockgen -source=pubsub_api.go -package mypubsub -destination pubsub_mock.go PubSub
type PubSub interface {
	Publish(c context.Context, topic string, data string) error
	CreateTopic(c context.Context, topic string) error
	Subscribe(c context.Context, topic string, urlToPostTo string) error
}

var New func(c context.Context) (PubSub, func(), error)
