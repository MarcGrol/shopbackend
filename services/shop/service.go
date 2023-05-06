package shop

import (
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
)

type service struct {
	basketStore mystore.Store[Basket]
	pubsub      mypubsub.PubSub
	publisher   mypublisher.Publisher
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(store mystore.Store[Basket], nower mytime.Nower, uuider myuuid.UUIDer, logger mylog.Logger, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) *service {
	return &service{
		basketStore: store,
		pubsub:      subscriber,
		publisher:   publisher,
		nower:       nower,
		uuider:      uuider,
		logger:      logger,
	}
}
