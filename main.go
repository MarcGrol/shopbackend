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
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/checkoutstripe"
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

	checkoutStore, checkoutStoreCleanup, err := mystore.New[checkoutapi.CheckoutContext](c)
	if err != nil {
		log.Fatalf("Error creating checkout store: %s", err)
	}
	defer checkoutStoreCleanup()

	oauthServiceCleanup := createOAuthService(c, router, vault, nower, uuider, eventPublisher)
	defer oauthServiceCleanup()

	createAdyenCheckoutService(c, router, checkoutStore, vault, nower, subscriber, eventPublisher)

	createStripeCheckoutService(c, router, checkoutStore, vault, nower, subscriber, eventPublisher)

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
		clientID := getenvOrAbort("STRIPE_OAUTH_CLIENT_ID")
		clientSecret := getenvOrAbort("STRIPE_OAUTH_CLIENT_SECRET")
		providers.Set("stripe", clientID, clientSecret, "", "")
	}

	oauthClient := oauthclient.NewOAuthClient(providers)
	oauthService := oauth.NewService(sessionStore, vault, nower, uuider, oauthClient, pub, providers)

	oauthService.RegisterEndpoints(c, router)

	return sessionStoreCleanup
}

func createAdyenCheckoutService(c context.Context, router *mux.Router, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader, nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) {

	merchantAccount := getenvOrAbort("ADYEN_MERCHANT_ACCOUNT")
	environment := getenvOrAbort("ADYEN_ENVIRONMENT")
	apiKey := getenvOrAbort("ADYEN_API_KEY")
	clientKey := getenvOrAbort("ADYEN_CLIENT_KEY")

	cfg := checkoutadyen.Config{
		Environment:     environment,
		MerchantAccount: merchantAccount,
		ClientKey:       clientKey,
		ApiKey:          apiKey,
	}

	payer := checkoutadyen.NewPayer(environment, apiKey)

	checkoutService, err := checkoutadyen.NewWebService(cfg, payer, checkoutStore, vault, nower, subscriber, publisher)
	if err != nil {
		log.Fatalf("Error creating adyen checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)
}

func createStripeCheckoutService(c context.Context, router *mux.Router, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader, nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) {

	apiKey := getenvOrAbort("STRIPE_API_KEY")

	payer := checkoutstripe.NewPayer()

	checkoutService, err := checkoutstripe.NewWebService(apiKey, payer, nower, checkoutStore, vault, publisher)
	if err != nil {
		log.Fatalf("Error creating stripe checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)
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
