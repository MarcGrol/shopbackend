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
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

type webService struct {
	logger  mylog.Logger
	service *service
}

// Use dependency injection to isolate the infrastructure and ease testing
func NewService(store mystore.Store[shopmodel.Basket], nower mytime.Nower, uuider myuuid.UUIDer, pub mypublisher.Publisher) *webService {
	logger := mylog.New("basket")
	return &webService{
		logger:  logger,
		service: newService(store, nower, uuider, logger),
	}
}

func (s webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	// Endpoints that compose the userinterface
	router.HandleFunc("/", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.basketListPage()).Methods("GET")
	router.HandleFunc("/basket", s.createNewBasketPage()).Methods("POST")
	router.HandleFunc("/basket/{basketUID}", s.basketDetailsPage()).Methods("GET")

	// Checkout component will redirect to this endpoint after checkout has finalized
	router.HandleFunc("/basket/{basketUID}/checkout/completed", s.checkoutFinalized()).Methods("GET")

	// Subsriptions arrive here as events
	router.HandleFunc("/api/basket/event", s.handleEventEnvelope()).Methods("POST")

	return s.service.subscribe(c)
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

func (s webService) handleEventEnvelope() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		envelope, err := mypublisher.ParseEventEnvelope(r.Body)
		if err != nil {
			errorWriter.WriteError(c, w, 4, myerrors.NewInvalidInputError(err))
			return
		}

		s.logger.Log(c, envelope.AggregateUID, mylog.SeverityInfo, "Received event envelope %+v", envelope)

		err = s.service.handleEvent(c, envelope)
		if err != nil {
			errorWriter.WriteError(c, w, 4, myerrors.NewInvalidInputError(err))
			return
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{
			Message: "Successfully processed token update",
		})
	}
}
