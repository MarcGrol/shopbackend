package oauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
)

type webService struct {
	service *service
	logger  mylog.Logger
}

func NewService(storer mystore.Store[Session], nower mytime.Nower, uuider myuuid.UUIDer, oauthClient OauthClient) *webService {
	return &webService{
		service: newService(storer, nower, uuider, oauthClient),
		logger:  mylog.New("oauth"),
	}
}

func (s webService) RegisterEndpoints(c context.Context, router *mux.Router) {
	router.HandleFunc("/oauth/start", s.startPage()).Methods("GET")
	router.HandleFunc("/oauth/done", s.donePage()).Methods("GET")
}

func (s webService) startPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		originalReturnURL := r.URL.Query().Get("returnURL")
		if originalReturnURL == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("Missing returnURL")))
			return
		}

		authenticationURL, err := s.service.start(c, originalReturnURL, myhttp.HostnameWithScheme(r))
		if err != nil {
			errorWriter.WriteError(c, w, 2, err)
			return
		}

		http.Redirect(w, r, authenticationURL, http.StatusSeeOther)
	}
}

func (s webService) donePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		sessionUID := r.URL.Query().Get("state")
		if sessionUID == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("Missing state")))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errorWriter.WriteError(c, w, 2, myerrors.NewInvalidInputError(fmt.Errorf("Missing code")))
			return
		}

		originalRedirectURL, err := s.service.done(c, sessionUID, code)
		if err != nil {
			errorWriter.WriteError(c, w, 3, myerrors.NewInvalidInputError(err))
			return
		}

		http.Redirect(w, r, originalRedirectURL, http.StatusSeeOther)
	}
}
