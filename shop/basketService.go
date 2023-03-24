package shop

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/myerrors"
	"github.com/MarcGrol/shopbackend/myhttp"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
	"github.com/MarcGrol/shopbackend/shop/store"
)

type service struct {
	basketStore store.BasketStorer
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewService(store store.BasketStorer) *service {
	return &service{
		basketStore: store,
	}
}

func (s service) RegisterEndpoints(c context.Context, router *mux.Router) {

	// Endpoints that compose the userinterface
	router.HandleFunc("/", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.createNewBasketPage()).Methods("POST")
	router.HandleFunc("/basket/{basketUID}", s.basketDetailsPage()).Methods("GET")

	// When checkout is completed, the user is redirected to this page
	router.HandleFunc("/basket/{basketUID}/checkout/completed", s.checkoutCompletedRedirectCallback()).Methods("GET")

	// Checkout component will call this endpoint to update the status of the checkout
	router.HandleFunc("/api/basket/{basketUID}/status/{eventCode}/{status}", s.checkoutStatusUpdate()).Methods("PUT")
}

//go:embed templates
var templateFolder embed.FS
var (
	basketListPageTemplate   *template.Template
	basketDetailPageTemplate *template.Template
)

func init() {
	basketListPageTemplate = template.Must(template.ParseFS(templateFolder, "templates/basket_list.html"))
	basketDetailPageTemplate = template.Must(template.ParseFS(templateFolder, "templates/basket_detail.html"))
}

func (s service) basketListPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		baskets, err := s.basketStore.List(c)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = basketListPageTemplate.Execute(w, baskets)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}
	}
}

func (s service) createNewBasketPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		uid := func() string { u, _ := uuid.NewUUID(); return u.String() }()
		returnURL := fmt.Sprintf("%s/basket/%s/checkout/completed", myhttp.HostnameWithScheme(r), uid)

		basket := createBasket(uid, returnURL)
		err := s.basketStore.Put(c, uid, &basket)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}

		// Back to the basket list
		http.Redirect(w, r, fmt.Sprintf("%s/basket", myhttp.HostnameWithScheme(r)), http.StatusSeeOther)
	}
}

func (s service) basketDetailsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		basketUID := mux.Vars(r)["basketUID"]
		basket, found, err := s.basketStore.Get(c, basketUID)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}
		if !found {
			myhttp.WriteError(w, 1, myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID)))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = basketDetailPageTemplate.Execute(w, basket)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}
	}
}

func (s service) checkoutCompletedRedirectCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		basketUID := mux.Vars(r)["basketUID"]
		status := r.URL.Query().Get("status")

		log.Printf("Checkout completed for basket %s -> %s", basketUID, status)

		basket, found, err := s.basketStore.Get(c, basketUID)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}
		if !found {
			myhttp.WriteError(w, 1, myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID)))
			return
		}

		basket.InitialPaymentStatus = status
		err = s.basketStore.Put(c, basketUID, &basket)
		if err != nil {
			myhttp.WriteError(w, 2, myerrors.NewInternalError(err))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		basketDetailPageTemplate.Execute(w, basket)
	}
}

func (s service) checkoutStatusUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := context.Background()

		basketUID := mux.Vars(r)["basketUID"]
		eventCode := mux.Vars(r)["eventCode"]
		status := mux.Vars(r)["status"]

		log.Printf("Checkout status update for basket %s (%s) -> %s", basketUID, eventCode, status)

		// TODO use a transaction

		basket, found, err := s.basketStore.Get(c, basketUID)
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}
		if !found {
			myhttp.WriteError(w, 1, myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID)))
			return
		}

		basket.FinalPaymentEvent = eventCode
		basket.FinalPaymentStatus = status
		err = s.basketStore.Put(c, basketUID, &basket)
		if err != nil {
			myhttp.WriteError(w, 2, myerrors.NewInternalError(err))
			return
		}
		myhttp.Write(w, http.StatusOK, myhttp.EmptyResponse{})
	}
}

func createBasket(orderRef string, returnURL string) shopmodel.Basket {
	return shopmodel.Basket{
		UID:        orderRef,
		CreatedAt:  time.Now(),
		Shop:       getCurrentShop(),
		Shopper:    getCurrentShopper(),
		TotalPrice: 51000,
		Currency:   "EUR",
		SelectedProducts: []shopmodel.SelectedProduct{
			{
				UID:         "product_tennis_racket",
				Description: "Tennis racket",
				Price:       10000,
				Currency:    "EUR",
				Quantity:    5,
			},
			{
				UID:         "product_tennis_balls",
				Description: "Tennis balls",
				Price:       1000,
				Currency:    "EUR",
				Quantity:    1,
			},
		},
		ReturnURL:            returnURL,
		InitialPaymentStatus: "open",
	}
}

func getCurrentShop() shopmodel.Shop {
	return shopmodel.Shop{
		UID:      "shop_evas_shop",
		Name:     "Eva's shop",
		Country:  "NL",
		Currency: "EUR",
		Hostname: "https://www.marcgrolconsultancy.nl/", // "http://localhost:8082"
	}
}

func getCurrentShopper() shopmodel.Shopper {
	return shopmodel.Shopper{
		UID:         "shopper_marc_grol",
		FirstName:   "Marc",
		LastName:    "Grol",
		DateOfBirth: func() *time.Time { t := time.Date(1971, time.February, 27, 0, 0, 0, 0, time.UTC); return &t }(),
		Address: shopmodel.Address{
			City:              "De Bilt",
			Country:           "NL",
			HouseNumberOrName: "79",
			PostalCode:        "3731TB",
			StateOrProvince:   "Utrecht",
			Street:            "Heemdstrakwartier",
		},
		Country:      "NL",
		Locale:       "nl-NL",
		EmailAddress: "marc.grol@gmail.com",
		PhoneNumber:  "+31648928856",
	}
}
