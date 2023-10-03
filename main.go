package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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
	"github.com/MarcGrol/shopbackend/services/checkoutmollie"
	"github.com/MarcGrol/shopbackend/services/checkoutstripe"
	"github.com/MarcGrol/shopbackend/services/oauth"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient/challenge"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthvault"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
	"github.com/MarcGrol/shopbackend/services/shop"
	"github.com/MarcGrol/shopbackend/services/termsconditions"
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

	vault, vaultCleanup, err := myvault.NewReaderWriter[oauthvault.Token](c)
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

	createMollieCheckoutService(c, router, checkoutStore, vault, nower, subscriber, eventPublisher)

	shopServiceCleanup := createShopService(c, router, nower, uuider, subscriber, eventPublisher)
	defer shopServiceCleanup()

	createWarmupService(c, router, vault, uuider, eventPublisher)

	createTermsConditionsService(c, router, eventPublisher)

	startWebServerBlocking(router)
}

func createShopService(c context.Context, router *mux.Router, nower mytime.Nower,
	uuider myuuid.UUIDer, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) func() {

	basketStore, basketstoreCleanup, err := mystore.New[shop.Basket](c)
	if err != nil {
		log.Fatalf("Error creating basket store: %s", err)
	}

	basketService := shop.NewService(basketStore, nower, uuider, subscriber, publisher)
	err = basketService.RegisterEndpoints(c, router)
	if err != nil {
		log.Fatalf("Error registering basket store: %s", err)
	}

	return basketstoreCleanup
}

func createOAuthService(c context.Context, router *mux.Router, vault myvault.VaultReadWriter[oauthvault.Token], nower mytime.Nower, uuider myuuid.UUIDer, pub mypublisher.Publisher) func() {
	partyStore, partyStoreCleanup, err := mystore.New[providers.OauthParty](c)
	if err != nil {
		log.Fatalf("Error creating oauth-party store: %s", err)
	}

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

	{
		clientID := getenvOrAbort("MOLLIE_OAUTH_CLIENT_ID")
		clientSecret := getenvOrAbort("MOLLIE_OAUTH_CLIENT_SECRET")
		providers.Set("mollie", clientID, clientSecret, "", "")
	}

	oauthClient := oauthclient.NewOAuthClient(providers, challenge.NewRandomStringer())
	oauthService := oauth.NewService(partyStore, sessionStore, vault, nower, uuider, oauthClient, pub, providers)

	err = oauthService.RegisterEndpoints(c, router)
	if err != nil {
		log.Fatalf("Error registering oauth service: %s", err)
	}

	return func() {
		sessionStoreCleanup()
		partyStoreCleanup()
	}
}

func createAdyenCheckoutService(c context.Context, router *mux.Router, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader[oauthvault.Token], nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) {

	merchantAccount := getenvOrAbort("ADYEN_MERCHANT_ACCOUNT")
	environment := getenvOrAbort("ADYEN_ENVIRONMENT")
	apiKey := getenvOrAbort("ADYEN_API_KEY")
	clientKey := getenvOrAbort("ADYEN_CLIENT_KEY")

	cfg := checkoutadyen.Config{
		Environment:     environment,
		MerchantAccount: merchantAccount,
		ClientKey:       clientKey,
		APIKey:          apiKey,
	}

	payer := checkoutadyen.NewPayer(environment, apiKey)

	checkoutService, err := checkoutadyen.NewWebService(cfg, payer, checkoutStore, vault, nower, subscriber, publisher)
	if err != nil {
		log.Fatalf("Error creating adyen checkoutService: %s", err)
	}

	err = checkoutService.RegisterEndpoints(c, router)
	if err != nil {
		log.Fatalf("Error registering adyen checkout service: %s", err)
	}
}

func createStripeCheckoutService(c context.Context, router *mux.Router, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader[oauthvault.Token], nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) {

	apiKey := getenvOrAbort("STRIPE_API_KEY")

	payer := checkoutstripe.NewPayer()

	checkoutService, err := checkoutstripe.NewWebService(apiKey, payer, nower, checkoutStore, vault, publisher)
	if err != nil {
		log.Fatalf("Error creating stripe checkoutService: %s", err)
	}

	err = checkoutService.RegisterEndpoints(c, router)
	if err != nil {
		log.Fatalf("Error registering stripe checkout service: %s", err)
	}
}

func createMollieCheckoutService(c context.Context, router *mux.Router, checkoutStore mystore.Store[checkoutapi.CheckoutContext], vault myvault.VaultReader[oauthvault.Token], nower mytime.Nower, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) {

	apiKey := getenvOrAbort("MOLLIE_API_KEY")

	payer, err := checkoutmollie.NewPayer()
	if err != nil {
		log.Fatalf("Error creating mollie payer: %s", err)
	}

	checkoutService, err := checkoutmollie.NewWebService(apiKey, payer, nower, checkoutStore, vault, publisher)
	if err != nil {
		log.Fatalf("Error creating mollie checkoutService: %s", err)
	}

	err = checkoutService.RegisterEndpoints(c, router)
	if err != nil {
		log.Fatalf("Error registering mollie checkout service: %s", err)
	}
}

func createWarmupService(c context.Context, router *mux.Router, vault myvault.VaultReader[oauthvault.Token], uuider myuuid.UUIDer, pub mypublisher.Publisher) {
	warmupService := warmup.NewService(vault, uuider, pub)
	err := warmupService.RegisterEndpoints(c, router)
	if err != nil {
		log.Fatalf("Error registering warmup service: %s", err)
	}
}

func createTermsConditionsService(c context.Context, router *mux.Router, pub mypublisher.Publisher) {
	service := termsconditions.NewService(pub)
	err := service.RegisterEndpoints(c, router)
	if err != nil {
		log.Fatalf("Error registering warmup service: %s", err)
	}
}

func startWebServerBlocking(router *mux.Router) {
	port := getenvWithDefault("PORT", "8080")

	log.Printf("Starting webserver on port %s (try http://localhost:%s)", port, port)

	srv := http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  5 * time.Second,
		IdleTimeout:  5 * time.Second,
		Handler:      http.TimeoutHandler(http.HandlerFunc(router.ServeHTTP), 10*time.Second, "Timeout!\n"),
	}

	err := srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Error starting webserver on port %s: %s", port, err)
	}
}

func getenvOrAbort(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("terminatiing because of missing mandatoty env-var %s.", name)
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
