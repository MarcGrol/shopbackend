package oauth

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/codeverifier"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/oauth/oauthevents"
)

const (
	exampleScopes = "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write"
)

type service struct {
	clientID    string
	storer      mystore.Store[OAuthSessionSetup]
	vault       myvault.VaultReadWriter
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
	oauthClient OauthClient
	publisher   mypublisher.Publisher
}

func newService(clientID string, storer mystore.Store[OAuthSessionSetup], vault myvault.VaultReadWriter, nower mytime.Nower, uuider myuuid.UUIDer, oauthClient OauthClient, pub mypublisher.Publisher) *service {
	return &service{
		clientID:    clientID,
		storer:      storer,
		vault:       vault,
		nower:       nower,
		uuider:      uuider,
		oauthClient: oauthClient,
		logger:      mylog.New("oauth"),
		publisher:   pub,
	}
}

func (s service) subscribe(c context.Context) error {
	client, cleanup, err := mypubsub.New(c)
	if err != nil {
		return fmt.Errorf("error creating client: %s", err)
	}
	defer cleanup()

	err = client.CreateTopic(c, oauthevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", oauthevents.TopicName, err)
	}

	return nil
}

func (s service) start(c context.Context, originalReturnURL string, hostname string) (string, error) {

	codeVerifier, err := codeverifier.NewVerifier()
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error creating verifier: %s", err))
	}
	codeVerifierValue := codeVerifier.GetValue()

	now := s.nower.Now()
	sessionUID := s.uuider.Create()

	authURL := ""
	err = s.storer.RunInTransaction(c, func(c context.Context) error {

		// Create new sessionx
		err := s.storer.Put(c, sessionUID, OAuthSessionSetup{
			UID:       sessionUID,
			ClientID:  s.clientID,
			Scopes:    exampleScopes,
			ReturnURL: originalReturnURL,
			Verifier:  codeVerifierValue,
			CreatedAt: now,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing: %s", err))
		}

		authURL, err = s.oauthClient.ComposeAuthURL(c, ComposeAuthURLRequest{
			CompletionURL: createCompletionURL(hostname), // Be called back here when authorisation has completed
			Scope:         exampleScopes,
			State:         sessionUID,
			CodeVerifier:  codeVerifierValue,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error composing auth url: %s", err))
		}

		err = s.publisher.Publish(c, oauthevents.TopicName, oauthevents.OAuthSessionSetupStarted{
			SessionUID: sessionUID,
			ClientID:   s.clientID,
			Scopes:     exampleScopes,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Started oauth session-setup %s", sessionUID)

	return authURL, nil
}

func (s service) done(c context.Context, sessionUID string, code string, hostname string) (string, error) {
	now := s.nower.Now()

	returnURL := ""
	tokenResp := GetTokenResponse{}
	err := s.storer.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		session, exist, err := s.storer.Get(c, sessionUID)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error fetching session: %s", err))
		}
		if !exist {
			return myerrors.NewNotFoundError(fmt.Errorf("OAuthSessionSetup with uid %s not found", sessionUID))
		}
		returnURL = session.ReturnURL

		// Get token
		tokenResp, err = s.oauthClient.GetAccessToken(c, GetTokenRequest{
			RedirectUri:  createCompletionURL(hostname),
			Code:         code,
			CodeVerifier: session.Verifier,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error getting token: %s", err))
		}

		s.logger.Log(c, sessionUID, mylog.SeverityDebug, "token-resp: %+v", tokenResp)

		// Update session
		session.TokenData = &tokenResp
		session.LastModified = &now
		session.Done = true
		err = s.storer.Put(c, sessionUID, session)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing session: %s", err))
		}

		// Store token in vault
		err = s.vault.Put(c, myvault.CurrentToken, myvault.Token{
			CreatedAt:    now,
			ClientID:     session.ClientID,
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			ExpiresIn:    tokenResp.ExpiresIn,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing token in vault: %s", err))
		}

		err = s.publisher.Publish(c, oauthevents.TopicName, oauthevents.OAuthSessionSetupCompleted{
			SessionUID: sessionUID,
			Success:    true,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Completed oauth session-setup %s", sessionUID)

	return returnURL, nil
}

func createCompletionURL(hostname string) string {
	return fmt.Sprintf("%s/oauth/done", hostname)
}

func (s service) refreshToken(c context.Context) error {

	now := s.nower.Now()
	uid := s.uuider.Create()

	err := s.storer.RunInTransaction(c, func(c context.Context) error {
		currentToken, exists, err := s.vault.Get(c, myvault.CurrentToken)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error fetching current token:%s", err))
		}

		if !exists {
			// cannot refreshToken without a token: do not consider this a failure
			return nil
		}

		newTokenResp, err := s.oauthClient.RefreshAccessToken(c, RefreshTokenRequest{
			RefreshToken: currentToken.RefreshToken,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error refreshing token: %s", err))
		}

		s.logger.Log(c, "", mylog.SeverityDebug, "refresh-token-resp: %+v", newTokenResp)

		// Update token
		err = s.vault.Put(c, myvault.CurrentToken, myvault.Token{
			UID:          uid,
			CreatedAt:    now,
			ClientID:     currentToken.ClientID,
			AccessToken:  newTokenResp.AccessToken,
			RefreshToken: newTokenResp.RefreshToken,
			ExpiresIn:    newTokenResp.ExpiresIn,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing token: %s", err))
		}

		err = s.publisher.Publish(c, oauthevents.TopicName, oauthevents.OAuthTokenRefreshCompleted{
			UID:      uid,
			ClientID: currentToken.ClientID,
			Success:  true,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		s.logger.Log(c, "", mylog.SeverityInfo, "Complete oauth session-refresh-token")

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
