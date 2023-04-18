package myevents

import "time"

type EventEnvelope struct {
	UID           string
	CreatedAt     time.Time
	Topic         string
	AggregateUID  string
	EventTypeName string
	EventPayload  string `datastore:",noindex"`
	Published     bool
}

func (e EventEnvelope) String() string {
	return e.Topic + "." + e.EventTypeName + "." + e.AggregateUID
}

type Event interface {
	GetEventTypeName() string
	GetAggregateName() string
}
