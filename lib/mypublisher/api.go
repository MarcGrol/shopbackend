package mypublisher

import (
	"context"

	"github.com/MarcGrol/shopbackend/lib/myevents"
)

//go:generate mockgen -source=api.go -package mypublisher -destination publisher_mock.go Publisher
type Publisher interface {
	Publish(c context.Context, topic string, env myevents.Event) error
}
