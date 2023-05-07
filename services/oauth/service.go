package oauth

import (
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
)

type service struct {
	storer      mystore.Store[OAuthSessionSetup]
	vault       myvault.VaultReadWriter
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
	oauthClient OauthClient
	publisher   mypublisher.Publisher
	providers   providers.OAuthProvider
}

func newService(storer mystore.Store[OAuthSessionSetup], vault myvault.VaultReadWriter, nower mytime.Nower, uuider myuuid.UUIDer, oauthClient OauthClient, pub mypublisher.Publisher, providers providers.OAuthProvider) *service {
	return &service{
		storer:      storer,
		vault:       vault,
		nower:       nower,
		uuider:      uuider,
		oauthClient: oauthClient,
		logger:      mylog.New("oauth"),
		publisher:   pub,
		providers:   providers,
	}
}
