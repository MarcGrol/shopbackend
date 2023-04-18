package oauth

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
	"github.com/MarcGrol/shopbackend/lib/myvault"
)

type webService struct {
	service *service
	logger  mylog.Logger
}

func NewService(clientID string, storer mystore.Store[OAuthSessionSetup], vault myvault.VaultReadWriter, nower mytime.Nower, uuider myuuid.UUIDer, oauthClient OauthClient, pub mypublisher.Publisher) *webService {
	return &webService{
		service: newService(clientID, storer, vault, nower, uuider, oauthClient, pub),
		logger:  mylog.New("oauth"),
	}
}

func (s *webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	router.HandleFunc("/oauth/admin", s.adminPage()).Methods("GET")

	router.HandleFunc("/oauth/start", s.startPage()).Methods("GET")
	router.HandleFunc("/oauth/done", s.donePage()).Methods("GET")
	router.HandleFunc("/oauth/refresh", s.refreshTokenPage()).Methods("GET")

	err := s.service.CreateTopics(context.Background())
	if err != nil {
		return err
	}

	return nil
}

//go:embed templates
var templateFolder embed.FS
var (
	adminPageTemplate *template.Template
)

func init() {
	adminPageTemplate = template.Must(template.ParseFS(templateFolder, "templates/admin.html"))
}

func (s *webService) adminPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		oauthStatus, err := s.service.getOauthStatus(c)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = adminPageTemplate.Execute(w, oauthStatus)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}
	}
}
func (s *webService) startPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		originalReturnURL := r.URL.Query().Get("returnURL")
		if originalReturnURL == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing returnURL")))
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

func (s *webService) donePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		sessionUID := r.URL.Query().Get("state")
		if sessionUID == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing state")))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errorWriter.WriteError(c, w, 2, myerrors.NewInvalidInputError(fmt.Errorf("missing code")))
			return
		}

		originalRedirectURL, err := s.service.done(c, sessionUID, code, myhttp.HostnameWithScheme(r))
		if err != nil {
			errorWriter.WriteError(c, w, 3, err)
			return
		}

		http.Redirect(w, r, originalRedirectURL, http.StatusSeeOther)
	}
}

func (s *webService) refreshTokenPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		err := s.service.refreshToken(c)
		if err != nil {
			errorWriter.WriteError(c, w, 4, err)
			return
		}

		oauthStatus, err := s.service.getOauthStatus(c)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = adminPageTemplate.Execute(w, oauthStatus)
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}
	}
}
