package oauth

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/codeverifier"
	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
)

const (
	exampleScope = "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write"
)

type webService struct {
	storer      mystore.Store[Session]
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
	oauthClient OauthClient
}

func NewService(storer mystore.Store[Session], nower mytime.Nower, uuider myuuid.UUIDer, tokenGetter OauthClient) *webService {
	return &webService{
		storer:      storer,
		nower:       nower,
		uuider:      uuider,
		oauthClient: tokenGetter,
		logger:      mylog.New("oauth"),
	}
}

func (s webService) RegisterEndpoints(c context.Context, router *mux.Router) {
	router.HandleFunc("/oauth/start", s.start()).Methods("GET")
	router.HandleFunc("/oauth/done", s.done()).Methods("GET")
}

func (s webService) start() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		originalReturnURL := r.URL.Query().Get("returnURL")
		if originalReturnURL == "" {
			errorWriter.WriteError(c, w, 1, myerrors.NewInvalidInputError(fmt.Errorf("Missing returnURL")))
			return
		}

		codeVerifier, err := codeverifier.NewVerifier()
		if err != nil {
			errorWriter.WriteError(c, w, 2, myerrors.NewInternalError(fmt.Errorf("error creating verifier: %s", err)))
		}
		codeVerifierValue := codeVerifier.GetValue()

		uid := s.uuider.Create()

		// Create new session
		err = s.storer.Put(c, uid, Session{
			UID:       uid,
			ReturnURL: originalReturnURL,
			Verifier:  codeVerifierValue,
		})
		if err != nil {
			errorWriter.WriteError(c, w, 3, myerrors.NewInternalError(fmt.Errorf("error storing: %s", err)))
			return
		}

		// Be called back here when authorisation has completed
		completionURL := fmt.Sprintf("%s/oauth/done", myhttp.HostnameWithScheme(r))

		authURL, err := s.oauthClient.ComposeAuthURL(c, ComposeAuthURLRequest{
			CompletionURL: completionURL,
			Scope:         exampleScope,
			State:         uid,
			CodeVerifier:  codeVerifierValue,
		})
		if err != nil {
			errorWriter.WriteError(c, w, 4, myerrors.NewInternalError(fmt.Errorf("error composing auth url: %s", err)))
			return
		}

		http.Redirect(w, r, authURL, http.StatusSeeOther)
	}
}

func (s webService) done() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		sessionUID := r.URL.Query().Get("state")
		if sessionUID == "" {
			errorWriter.WriteError(c, w, 5, myerrors.NewInvalidInputError(fmt.Errorf("Missing state")))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errorWriter.WriteError(c, w, 6, myerrors.NewInvalidInputError(fmt.Errorf("Missing code")))
			return
		}

		returnURL := ""
		err := s.storer.RunInTransaction(c, func(c context.Context) error {
			session, exist, err := s.storer.Get(c, sessionUID)
			if err != nil {
				return myerrors.NewInternalError(fmt.Errorf("Error fetching session: %s", err))
			}
			if !exist {
				return myerrors.NewNotFoundError(fmt.Errorf("Session with uid %s not found", sessionUID))
			}
			returnURL = session.ReturnURL

			// Get token
			tokenResp, err := s.oauthClient.GetAccessToken(c, GetTokenRequest{
				RedirectUri:  session.ReturnURL,
				Code:         code,
				CodeVerifier: session.Verifier,
			})
			if err != nil {
				return myerrors.NewInternalError(fmt.Errorf("Error getting token: %s", err))
			}

			// Store tokens
			session.TokenData = tokenResp
			err = s.storer.Put(c, sessionUID, session)
			if err != nil {
				return myerrors.NewInternalError(fmt.Errorf("Error storing session: %s", err))
			}

			log.Printf("token-resp: %+v", tokenResp)

			return nil
		})
		if err != nil {
			errorWriter.WriteError(c, w, 7, myerrors.NewInvalidInputError(err))
			return
		}

		http.Redirect(w, r, returnURL, http.StatusSeeOther)
	}
}
