package oauth

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MarcGrol/shopbackend/lib/codeverifier"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
)

const (
	exampleScope = "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write"
)

type service struct {
	storer      mystore.Store[Session]
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
	oauthClient OauthClient
}

func newService(storer mystore.Store[Session], nower mytime.Nower, uuider myuuid.UUIDer, oauthClient OauthClient) *service {
	return &service{
		storer:      storer,
		nower:       nower,
		uuider:      uuider,
		oauthClient: oauthClient,
		logger:      mylog.New("oauth"),
	}
}

func (s service) start(c context.Context, originalReturnURL string, hostname string) (string, error) {

	codeVerifier, err := codeverifier.NewVerifier()
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error creating verifier: %s", err))
	}
	codeVerifierValue := codeVerifier.GetValue()

	sessionUID := s.uuider.Create()

	// Create new session
	err = s.storer.Put(c, sessionUID, Session{
		UID:       sessionUID,
		ReturnURL: originalReturnURL,
		Verifier:  codeVerifierValue,
		CreatedAt: s.nower.Now(),
	})
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error storing: %s", err))
	}

	// Be called back here when authorisation has completed
	completionURL := fmt.Sprintf("%s/oauth/done", hostname)

	authURL, err := s.oauthClient.ComposeAuthURL(c, ComposeAuthURLRequest{
		CompletionURL: completionURL,
		Scope:         exampleScope,
		State:         sessionUID,
		CodeVerifier:  codeVerifierValue,
	})
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error composing auth url: %s", err))
	}

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Start oauth session-setup %s", sessionUID)

	return authURL, nil
}

func (s service) done(c context.Context, sessionUID string, code string) (string, error) {
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
		session.LastModified = func() *time.Time { t := s.nower.Now(); return &t }()
		err = s.storer.Put(c, sessionUID, session)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("Error storing session: %s", err))
		}

		log.Printf("token-resp: %+v", tokenResp)

		return nil
	})
	if err != nil {
		return "", myerrors.NewInvalidInputError(err)
	}

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Complete oauth session-setup %s", sessionUID)

	return returnURL, nil
}
