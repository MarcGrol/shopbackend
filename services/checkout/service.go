package checkout

import (
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkout/checkoutmodel"
)

type service struct {
	environment     string
	merchantAccount string
	clientKey       string
	apiKey          string
	payer           Payer
	checkoutStore   mystore.Store[checkoutmodel.CheckoutContext]
	vault           myvault.VaultReader
	nower           mytime.Nower
	logger          mylog.Logger
	publisher       mypublisher.Publisher
}

// Use dependency injection to isolate the infrastructure and easy testing
func newCommandService(cfg Config, payer Payer, checkoutStorer mystore.Store[checkoutmodel.CheckoutContext], vault myvault.VaultReader, nower mytime.Nower, logger mylog.Logger, pub mypublisher.Publisher) (*service, error) {
	return &service{
		merchantAccount: cfg.MerchantAccount,
		environment:     cfg.Environment,
		clientKey:       cfg.ClientKey,
		apiKey:          cfg.ApiKey,
		payer:           payer,
		checkoutStore:   checkoutStorer,
		vault:           vault,
		nower:           nower,
		logger:          logger,
		publisher:       pub,
	}, nil
}
