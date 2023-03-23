package main

import (
	"context"
	"fmt"
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

	checkoutStore := checkout.NewCheckoutStore()
	checkoutService, err := checkout.NewService(checkoutStore)
	if err != nil {
		log.Fatalf("Error creating payment checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)

	basketStore := shop.NewBasketStore()
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
