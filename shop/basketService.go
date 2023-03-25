package shop

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/mycontext"
	"github.com/MarcGrol/shopbackend/myerrors"
	"github.com/MarcGrol/shopbackend/myhttp"
	"github.com/MarcGrol/shopbackend/mylog"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
	"github.com/MarcGrol/shopbackend/shop/store"
)

type service struct {
	basketStore store.BasketStorer
	logger      mylog.Logger
}

// Use dependency injection to isolate the infrastructure and easy testing
func NewService(store store.BasketStorer, logger mylog.Logger) *service {
	return &service{
		basketStore: store,
		logger:      logger,
	}
}

func (s service) RegisterEndpoints(c context.Context, router *mux.Router) {

	// Endpoints that compose the userinterface
	router.HandleFunc("/", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.createNewBasketPage()).Methods("POST")
	router.HandleFunc("/basket/{basketUID}", s.basketDetailsPage()).Methods("GET")
	router.HandleFunc("/basket/{basketUID}/checkout/completed", s.checkoutCompletedRedirectCallback()).Methods("GET")

	// Checkout component will call this endpoint to update the status of the checkout
	router.HandleFunc("/api/basket/{basketUID}/status/{eventCode}/{status}", s.checkoutStatusWebhookCallback()).Methods("PUT")
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
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		s.logger.Log(c, "", mylog.SeverityInfo, "Fetch all baskets")

		baskets, err := s.basketStore.List(c)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = basketListPageTemplate.Execute(w, baskets)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}
	}
}

func (s service) createNewBasketPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		uid := func() string { u, _ := uuid.NewUUID(); return u.String() }()
		returnURL := fmt.Sprintf("%s/basket/%s/checkout/completed", myhttp.HostnameWithScheme(r), uid)

		s.logger.Log(c, uid, mylog.SeverityInfo, "Creating new basket with uid %s", uid)

		basket := createBasket(uid, returnURL)
		err := s.basketStore.Put(c, uid, &basket)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}

		// Back to the basket list
		http.Redirect(w, r, fmt.Sprintf("%s/basket", myhttp.HostnameWithScheme(r)), http.StatusSeeOther)
	}
}

func (s service) basketDetailsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]

		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Fetch details of basket uid %s", basketUID)

		basket, found, err := s.basketStore.Get(c, basketUID)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}
		if !found {
			errorWriter.WriteError(c, w, 1, myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID)))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = basketDetailPageTemplate.Execute(w, basket)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}
	}
}

func (s service) checkoutCompletedRedirectCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]
		status := r.URL.Query().Get("status")

		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Redirect: Checkout completed for basket %s -> %s", basketUID, status)

		var basket *shopmodel.Basket
		var found bool
		var err error
		err = s.basketStore.RunInTransaction(c, func(c context.Context) error {

			basket, found, err = s.basketStore.Get(c, basketUID)
			if err != nil {
				return myerrors.NewInternalError(err)
			}
			if !found {
				return myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID))
			}

			basket.InitialPaymentStatus = status

			err = s.basketStore.Put(c, basketUID, basket)
			if err != nil {
				return myerrors.NewInternalError(err)
			}

			return nil
		})
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		basketDetailPageTemplate.Execute(w, basket)
	}
}

func (s service) checkoutStatusWebhookCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]
		eventCode := mux.Vars(r)["eventCode"]
		status := mux.Vars(r)["status"]

		s.logger.Log(c, basketUID, mylog.SeverityInfo, "Webhook: Checkout status update on basket %s (%s) -> %s", basketUID, eventCode, status)

		var basket *shopmodel.Basket
		var found bool
		var err error
		err = s.basketStore.RunInTransaction(c, func(c context.Context) error {
			basket, found, err = s.basketStore.Get(c, basketUID)
			if err != nil {
				return myerrors.NewInternalError(err)
			}
			if !found {
				return myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID))
			}

			// Final codes matter!
			basket.FinalPaymentEvent = eventCode
			basket.FinalPaymentStatus = status

			err = s.basketStore.Put(c, basketUID, basket)
			if err != nil {
				return myerrors.NewInternalError(err)
			}
			return nil
		})
		if err != nil {
			errorWriter.WriteError(c, w, 3, myerrors.NewInternalError(err))
			return
		}

		// This could be the place where a basket is being converted into an order

		errorWriter.Write(c, w, http.StatusOK, myhttp.EmptyResponse{})
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
	uid, _ := uuid.NewRandom()
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
		EmailAddress: fmt.Sprintf("marc.grol+%s@gmail.com", uid.String()),
		PhoneNumber:  "+31648928856",
	}
}
