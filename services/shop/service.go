package shop

import (
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/services/shop/shopmodel"
)

type service struct {
	basketStore mystore.Store[shopmodel.Basket]
	publisher   mypublisher.Publisher
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(store mystore.Store[shopmodel.Basket], nower mytime.Nower, uuider myuuid.UUIDer, logger mylog.Logger, pub mypublisher.Publisher) *service {
	return &service{
		basketStore: store,
		publisher:   pub,
		nower:       nower,
		uuider:      uuider,
		logger:      logger,
	}
}
