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

	checkoutServiceCleanup := createCheckoutService(c, router, queue)
	defer checkoutServiceCleanup()

	shopServiceCleanup := createShopService(c, router)
	defer shopServiceCleanup()

	startWebServerBlocking(router)
}

func createShopService(c context.Context, router *mux.Router) func() {

	basketStore, basketstoreCleanup, err := mystore.New[shopmodel.Basket](c)
	if err != nil {
		log.Fatalf("Error creating basket store: %s", err)
	}

	basketService := shop.NewService(basketStore, mylog.New("basket"))
	basketService.RegisterEndpoints(c, router)

	return basketstoreCleanup
}

func createCheckoutService(c context.Context, router *mux.Router, queue myqueue.TaskQueuer) func() {

	checkoutStore, checkoutStoreCleanup, err := mystore.New[checkoutmodel.CheckoutContext](c)
	if err != nil {
		log.Fatalf("Error creating checkout store: %s", err)
	}

	checkoutService, err := checkout.NewService(checkoutStore, queue, mylog.New("checkout"))
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
