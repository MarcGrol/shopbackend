package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/checkout"
	checkoutstore "github.com/MarcGrol/shopbackend/checkout/store"
	"github.com/MarcGrol/shopbackend/myhttpclient"
	"github.com/MarcGrol/shopbackend/shop"
	basketstore "github.com/MarcGrol/shopbackend/shop/store"
)

func main() {
	c := context.Background()

	router := mux.NewRouter()

	checkoutStore, checkoutStoreCleanup, err := checkoutstore.New(c)
	if err != nil {
		log.Fatalf("Error creating checkout store: %s", err)
	}
	defer checkoutStoreCleanup()

	httpClient := myhttpclient.New()
	checkoutService, err := checkout.NewService(checkoutStore, httpClient)
	if err != nil {
		log.Fatalf("Error creating payment checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)

	basketStore, basketstoreCleanup, err := basketstore.New(c)
	if err != nil {
		log.Fatalf("Error creating basket store: %s", err)
	}
	defer basketstoreCleanup()

	basketService := shop.NewService(basketStore)
	basketService.RegisterEndpoints(c, router)

	startWebServerBlocking(router)
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
