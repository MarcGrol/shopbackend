package main

import (
	"context"
	"fmt"
	"github.com/MarcGrol/shopbackend/checkout/store"
	"github.com/MarcGrol/shopbackend/myhttpclient"
	store2 "github.com/MarcGrol/shopbackend/shop/store"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/checkout"
	"github.com/MarcGrol/shopbackend/shop"
)

func main() {
	c := context.Background()

	router := mux.NewRouter()

	checkoutStore := store.NewCheckoutStore()
	httpClient := myhttpclient.New()
	checkoutService, err := checkout.NewService(checkoutStore, httpClient)
	if err != nil {
		log.Fatalf("Error creating payment checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)

	basketStore := store2.NewInMemoryBasketStore()
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
