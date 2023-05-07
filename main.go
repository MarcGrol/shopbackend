package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutadyen"
	"github.com/MarcGrol/shopbackend/services/oauth"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
	"github.com/MarcGrol/shopbackend/services/shop"
	"github.com/MarcGrol/shopbackend/services/warmup"
)

func main() {
	c := context.Background()
	router := mux.NewRouter()
	nower := mytime.RealNower{}
	uuider := myuuid.RealUUIDer{}

	queue, queueCleanup, err := myqueue.New(c)
	if err != nil {
		log.Fatalf("Error creating queue: %s", err)
	}
	defer queueCleanup()

	subscriber, pubsubCleanup, err := mypubsub.New(c)
	if err != nil {
		log.Fatalf("Error creating pubsub: %s", err)
	}
	defer pubsubCleanup()

	eventPublisher, eventPublisherCleanup, err := mypublisher.New(c, subscriber, queue, nower)
	if err != nil {
		log.Fatalf("Error creating event publisher: %s", err)
	}
	defer eventPublisherCleanup()
	eventPublisher.RegisterEndpoints(c, router)

	vault, vaultCleanup, err := myvault.New(c)
	if err != nil {
		log.Fatalf("Error creating vault: %s", err)
	}
	defer vaultCleanup()

	oauthServiceCleanup := createOAuthService(c, router, vault, nower, uuider, eventPublisher)
	defer oauthServiceCleanup()

	checkoutServiceCleanup := createCheckoutService(c, router, vault, nower, subscriber, eventPublisher)
	defer checkoutServiceCleanup()

	shopServiceCleanup := createShopService(c, router, nower, uuider, subscriber, eventPublisher)
	defer shopServiceCleanup()

	createWarmupService(c, router, vault, uuider, eventPublisher)

	startWebServerBlocking(router)
}

func createShopService(c context.Context, router *mux.Router, nower mytime.Nower,
	uuider myuuid.UUIDer, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) func() {

	basketStore, basketstoreCleanup, err := mystore.New[shop.Basket](c)
	if err != nil {
		log.Fatalf("Error creating basket store: %s", err)
	}

	basketService := shop.NewService(basketStore, nower, uuider, subscriber, publisher)
	basketService.RegisterEndpoints(c, router)

	return basketstoreCleanup
}

func createOAuthService(c context.Context, router *mux.Router, vault myvault.VaultReadWriter, nower mytime.Nower, uuider myuuid.UUIDer, pub mypublisher.Publisher) func() {
	sessionStore, sessionStoreCleanup, err := mystore.New[oauth.OAuthSessionSetup](c)
	if err != nil {
		log.Fatalf("Error creating oauth-session store: %s", err)
	}

	providers := providers.NewProviders()

	{
		clientID := getenvOrAbort("ADYEN_OAUTH_CLIENT_ID")
		clientSecret := getenvOrAbort("ADYEN_OAUTH_CLIENT_SECRET")
		providers.Set("adyen", clientID, clientSecret, "", "")
	}

	{
		stripeOAuthClientID := getenvOrAbort("STRIPE_OAUTH_CLIENT_ID")
		stripeOAuthClientSecret := getenvOrAbort("STRIPE_OAUTH_CLIENT_SECRET")
		providers.Set("stripe", stripeOAuthClientID, stripeOAuthClientSecret, "", "")
	}

	oauthClient := oauthclient.NewOAuthClient(providers)
	oauthService := oauth.NewService(sessionStore, vault, nower, uuider, oauthClient, pub, providers)

	oauthService.RegisterEndpoints(c, router)

	return sessionStoreCleanup
}

func createCheckoutService(c context.Context, router *mux.Router, vault myvault.VaultReader, nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) func() {

	merchantAccount := getenvOrAbort("ADYEN_MERCHANT_ACCOUNT")
	environment := getenvOrAbort("ADYEN_ENVIRONMENT")
	apiKey := getenvOrAbort("ADYEN_API_KEY")
	clientKey := getenvOrAbort("ADYEN_CLIENT_KEY")

	checkoutStore, checkoutStoreCleanup, err := mystore.New[checkoutadyen.CheckoutContext](c)
	if err != nil {
		log.Fatalf("Error creating checkout store: %s", err)
	}
	cfg := checkoutadyen.Config{
		Environment:     environment,
		MerchantAccount: merchantAccount,
		ClientKey:       clientKey,
		ApiKey:          apiKey,
	}

	payer := checkoutadyen.NewPayer(environment, apiKey)

	checkoutService, err := checkoutadyen.NewWebService(cfg, payer, checkoutStore, vault, nower, subscriber, publisher)
	if err != nil {
		log.Fatalf("Error creating payment checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)

	return checkoutStoreCleanup
}

func createWarmupService(c context.Context, router *mux.Router, vault myvault.VaultReader, uuider myuuid.UUIDer, pub mypublisher.Publisher) {
	warmupService := warmup.NewService(vault, uuider, pub)
	warmupService.RegisterEndpoints(c, router)
}

func startWebServerBlocking(router *mux.Router) {
	port := getenvWithDefault("PORT", "8080")

	log.Printf("Starting webserver on port %s (try http://localhost:%s)", port, port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), router)
	if err != nil {
		log.Fatalf("Error starting webserver on port %s: %s", port, err)
	}
}

func getenvOrAbort(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("missing env-var %s", name)
	}
	return value
}

func getenvWithDefault(name string, valueWhenNotSet string) string {
	value := os.Getenv(name)
	if value == "" {
		value = valueWhenNotSet
	}
	return value
}
