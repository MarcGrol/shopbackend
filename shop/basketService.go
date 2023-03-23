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
)

type service struct {
	basketStore BasketStore
}

func NewService(store BasketStore) *service {
	return &service{
		basketStore: store,
	}
}

func (s service) RegisterEndpoints(c context.Context, router *mux.Router) {

	router.HandleFunc("/", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.createNewBasketPage()).Methods("POST")
	router.HandleFunc("/basket/{basketUID}", s.basketDetailsPage()).Methods("GET")

	router.HandleFunc("/basket/{basketUID}/checkout/completed", s.checkoutCompletedCallback()).Methods("GET")
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
		baskets, err := s.basketStore.List()
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
		returnURL := fmt.Sprintf("http://%s/basket/%s/checkout/completed", r.Host, uid)

		err := s.basketStore.Put(c, uid, createBasket(uid, returnURL))
		if err != nil {
			myhttp.WriteError(w, 1, myerrors.NewInternalError(err))
			return
		}

		// Back to the bucket list
		http.Redirect(w, r, fmt.Sprintf("http://%s/basket", r.Host), http.StatusSeeOther)
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
			myhttp.WriteError(w, 1, myerrors.NewNotFoundError(fmt.Errorf("Basket with uid %s not found", basketUID)))
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

func (s service) checkoutCompletedCallback() http.HandlerFunc {
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
			myhttp.WriteError(w, 1, myerrors.NewNotFoundError(fmt.Errorf("Basket with uid %s not found", basketUID)))
			return
		}

		basket.PaymentStatus = status
		err = s.basketStore.Put(c, basketUID, basket)
		if err != nil {
			myhttp.WriteError(w, 2, myerrors.NewInternalError(err))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		basketDetailPageTemplate.Execute(w, basket)
	}
}

func createBasket(orderRef string, returnURL string) Basket {
	return Basket{
		UID:        orderRef,
		Shop:       getCurrentShop(),
		Shopper:    getCurrentShopper(),
		TotalPrice: 51000,
		Currency:   "EUR",
		SelectedProducts: []SelectedProduct{
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
		ReturnURL:     returnURL,
		PaymentStatus: "open",
	}
}

func getCurrentShop() Shop {
	return Shop{
		UID:             "shop_evas_shop",
		Name:            "Eva's shop",
		Country:         "NL",
		Currency:        "EUR",
		MerchantAccount: "MarcGrolConsultancyECOM",
		Hostname:        "http://localhost:8082", // "https://www.marcgrolconsultancy.nl/",
	}
}

func getCurrentShopper() Shopper {
	return Shopper{
		UID:         "shopper_marc_grol",
		FirstName:   "Marc",
		LastName:    "Grol",
		DateOfBirth: func() *time.Time { t := time.Date(1971, time.February, 27, 0, 0, 0, 0, time.UTC); return &t }(),
		Address: Address{
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
