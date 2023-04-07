package shop

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

type webService struct {
	service *service
	logger  mylog.Logger
}

// Use dependency injection to isolate the infrastructure and ease testing
func NewService(store mystore.Store[shopmodel.Basket], nower mytime.Nower, uuider myuuid.UUIDer, logger mylog.Logger) *webService {
	return &webService{
		service: newService(store, nower, uuider, logger),
		logger:  logger,
	}
}

func (s webService) RegisterEndpoints(c context.Context, router *mux.Router) {
	// Endpoints that compose the userinterface
	router.HandleFunc("/", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.createNewBasketPage()).Methods("POST")
	router.HandleFunc("/basket/{basketUID}", s.basketDetailsPage()).Methods("GET")

	// Checkout component will redirect to this endpoint after checkout has finalized
	router.HandleFunc("/basket/{basketUID}/checkout/completed", s.checkoutFinalized()).Methods("GET")

	// Checkout component will call this endpoint to update the status of the checkout
	router.HandleFunc("/api/basket/{basketUID}/status/{eventCode}/{status}", s.checkoutFinalStatusWebhook()).Methods("PUT")
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

func (s webService) basketListPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		baskets, err := s.service.listBaskets(c)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
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

func (s webService) createNewBasketPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basket, err := s.service.createNewBasket(c, myhttp.HostnameWithScheme(r))
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}

		// Redirect to newly created basket
		http.Redirect(w, r, fmt.Sprintf("%s/basket/%s", myhttp.HostnameWithScheme(r), basket.UID), http.StatusSeeOther)
	}
}

func (s webService) basketDetailsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]

		basket, err := s.service.getBasket(c, basketUID)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
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

func (s webService) checkoutFinalized() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]
		status := r.URL.Query().Get("status")

		basket, err := s.service.checkoutFinalized(c, basketUID, status)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		basketDetailPageTemplate.Execute(w, basket)
	}
}

func (s webService) checkoutFinalStatusWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		basketUID := mux.Vars(r)["basketUID"]
		eventCode := mux.Vars(r)["eventCode"]
		status := mux.Vars(r)["status"]

		err := s.service.checkoutFinalStatusWebhook(c, basketUID, eventCode, status)
		if err != nil {
			errorWriter.WriteError(c, w, 3, myerrors.NewInternalError(err))
			return
		}

		// This could be the place where a basket is being converted into an order

		errorWriter.Write(c, w, http.StatusOK, myhttp.EmptyResponse{})
	}
}
