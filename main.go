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
	"github.com/MarcGrol/shopbackend/experiment"
	"github.com/MarcGrol/shopbackend/mylog"
	"github.com/MarcGrol/shopbackend/myqueue"
	"github.com/MarcGrol/shopbackend/shop"
	basketstore "github.com/MarcGrol/shopbackend/shop/store"
)

func main() {
	c := context.Background()

	router := mux.NewRouter()

	personStore, personStoreCleanup, err := experiment.New[experiment.Person](c)
	if err != nil {
		log.Fatalf("Error creating queue: %s", err)
	}
	defer personStoreCleanup()
	personService := experiment.NewPersonService(personStore)
	personService.RegisterEndpoints(c, router)

	queue, queueCleanup, err := myqueue.New(c)
	if err != nil {
		log.Fatalf("Error creating queue: %s", err)
	}
	defer queueCleanup()

	checkoutStore, checkoutStoreCleanup, err := checkoutstore.New(c)
	if err != nil {
		log.Fatalf("Error creating checkout store: %s", err)
	}
	defer checkoutStoreCleanup()

	checkoutService, err := checkout.NewService(checkoutStore, queue, mylog.New("checkout"))
	if err != nil {
		log.Fatalf("Error creating payment checkoutService: %s", err)
	}
	checkoutService.RegisterEndpoints(c, router)

	basketStore, basketstoreCleanup, err := basketstore.New(c)
	if err != nil {
		log.Fatalf("Error creating basket store: %s", err)
	}
	defer basketstoreCleanup()

	basketService := shop.NewService(basketStore, mylog.New("basket"))
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
