package mypubsub

import (
	"context"
	"time"
)

type EventEnvelope struct {
	UID           string
	CreatedAt     time.Time
	Topic         string
	EventTypeName string
	EventPayload  string `datastore:",noindex"`
	Published     bool
}

type Event interface {
	GetEventTypeName() string
}

//go:generate mockgen -source=api.go -package mypubsub -destination publisher_mock.go Publisher
type Publisher interface {
	Publish(c context.Context, topic string, env Event) error
}
