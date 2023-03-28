package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/checkout"
	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/shop"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

func main() {
	c := context.Background()

	router := mux.NewRouter()

	queue, queueCleanup, err := myqueue.New(c)
	if err != nil {
		log.Fatalf("Error creating queue: %s", err)
	}
	defer queueCleanup()

	nower := mytime.RealNower{}
	uuider := myuuid.RealUUIDer{}

	checkoutServiceCleanup := createCheckoutService(c, router, queue, nower, uuider)
	defer checkoutServiceCleanup()

	shopServiceCleanup := createShopService(c, router, nower, uuider)
	defer shopServiceCleanup()

	startWebServerBlocking(router)
}

func createShopService(c context.Context, router *mux.Router, nower mytime.Nower,
	uuider myuuid.UUIDer) func() {

	basketStore, basketstoreCleanup, err := mystore.New[shopmodel.Basket](c)
	if err != nil {
		log.Fatalf("Error creating basket store: %s", err)
	}

	basketService := shop.NewService(basketStore, nower, uuider, mylog.New("basket"))
	basketService.RegisterEndpoints(c, router)

	return basketstoreCleanup
}

const (
	merchantAccountVarname = "ADYEN_MERCHANT_ACCOUNT"
	apiKeyVarname          = "ADYEN_API_KEY"
	clientKeyVarname       = "ADYEN_CLIENT_KEY"
	environmentVarname     = "ADYEN_ENVIRONMENT"
)

func createCheckoutService(c context.Context, router *mux.Router, queue myqueue.TaskQueuer,
	nower mytime.Nower, uuider myuuid.UUIDer) func() {

	merchantAccount := os.Getenv(merchantAccountVarname)
	if merchantAccount == "" {
		log.Fatalf("missing env-var %s", merchantAccountVarname)
	}

	environment := os.Getenv(environmentVarname)
	if environment == "" {
		log.Fatalf("missing env-var %s", environmentVarname)
	}

	apiKey := os.Getenv(apiKeyVarname)
	if apiKey == "" {
		log.Fatalf("missing env-var %s", apiKeyVarname)
	}

	clientKey := os.Getenv(clientKeyVarname)
	if clientKey == "" {
		log.Fatalf("missing env-var %s", clientKeyVarname)
	}

	checkoutStore, checkoutStoreCleanup, err := mystore.New[checkoutmodel.CheckoutContext](c)
	if err != nil {
		log.Fatalf("Error creating checkout store: %s", err)
	}
	cfg := checkout.Config{
		Environment:     environment,
		MerchantAccount: merchantAccount,
		ClientKey:       clientKey,
		ApiKey:          apiKey,
	}

	payer := checkout.NewPayer(environment, apiKey)

	checkoutService, err := checkout.NewService(cfg, payer, checkoutStore, queue, nower, mylog.New("checkout"))
	if err != nil {
		log.Fatalf("Error creating payment checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)

	return checkoutStoreCleanup
}

func startWebServerBlocking(router *mux.Router) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting webserver on port %s (try http://localhost:%s)", port, port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), router)
	if err != nil {
		log.Fatalf("Error starting webserver on port %s: %s", port, err)
	}
}
