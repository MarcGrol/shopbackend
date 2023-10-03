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
	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthvault"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
)

type webService struct {
	service *service
	logger  mylog.Logger
}

func NewService(partyVault myvault.VaultReadWriter[providers.OauthParty], sessionStore mystore.Store[OAuthSessionSetup], vault myvault.VaultReadWriter[oauthvault.Token], nower mytime.Nower, uuider myuuid.UUIDer, oauthClient oauthclient.OauthClient, pub mypublisher.Publisher, providers providers.OAuthProvider) *webService {
	return &webService{
		service: newService(partyVault, sessionStore, vault, nower, uuider, oauthClient, pub, providers),
		logger:  mylog.New("oauth"),
	}
}

func (s *webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	router.HandleFunc("/oauth/admin", s.adminPage()).Methods("GET")

	router.HandleFunc("/oauth/start/{providerName}", s.startPage()).Methods("POST")
	router.HandleFunc("/oauth/done", s.donePage()).Methods("GET")
	router.HandleFunc("/oauth/refresh/{providerName}", s.refreshTokenPage()).Methods("GET")  // cron support onnly get
	router.HandleFunc("/oauth/refresh/{providerName}", s.refreshTokenPage()).Methods("POST") // as used from screens
	router.HandleFunc("/oauth/cancel/{providerName}", s.cancelTokenPage()).Methods("POST")

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

		oauthStatuses, err := s.service.getOauthStatus(c)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = adminPageTemplate.Execute(w, oauthStatuses)
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

		providerName := mux.Vars(r)["providerName"]
		if providerName == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing providerName")))
			return
		}

		err := r.ParseForm()
		if err != nil {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(err))
			return
		}

		requestedScopes := r.FormValue("scopes")

		originalReturnURL := r.FormValue("returnURL")
		if originalReturnURL == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing returnURL")))
			return
		}

		clientID := r.FormValue("clientID")
		if clientID == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing clientID")))
			return
		}

		clientSecret := r.FormValue("clientSecret")
		if clientSecret == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing clientSecret")))
			return
		}

		authenticationURL, err := s.service.start(c, providerName, clientID, clientSecret,
			requestedScopes, originalReturnURL, myhttp.HostnameWithScheme(r))
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

		error := r.URL.Query().Get("error")
		if error != "" {
			errorDescription := r.URL.Query().Get("error_description")
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("%s (%s)", error, errorDescription)))
			return
		}

		sessionUID := r.URL.Query().Get("state")
		if sessionUID == "" {
			errorWriter.WriteError(c, w, 2, myerrors.NewInvalidInputError(fmt.Errorf("missing state")))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errorWriter.WriteError(c, w, 3, myerrors.NewInvalidInputError(fmt.Errorf("missing code")))
			return
		}

		originalRedirectURL, err := s.service.done(c, sessionUID, code, myhttp.HostnameWithScheme(r))
		if err != nil {
			errorWriter.WriteError(c, w, 4, err)
			return
		}

		http.Redirect(w, r, originalRedirectURL, http.StatusSeeOther)
	}
}

func (s *webService) refreshTokenPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		providerName := mux.Vars(r)["providerName"]
		if providerName == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing providerName")))
			return
		}

		_, err := s.service.refreshToken(c, providerName)
		if err != nil {
			errorWriter.WriteError(c, w, 4, err)
			return
		}

		http.Redirect(w, r, "/oauth/admin", http.StatusSeeOther)
	}
}

func (s *webService) cancelTokenPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		providerName := mux.Vars(r)["providerName"]
		if providerName == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("missing providerName")))
			return
		}

		err := s.service.cancelToken(c, providerName)
		if err != nil {
			errorWriter.WriteError(c, w, 4, err)
			return
		}

		http.Redirect(w, r, "/oauth/admin", http.StatusSeeOther)
	}
}
