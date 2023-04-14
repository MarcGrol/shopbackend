package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcGrol/shopbackend/lib/codeverifier"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
)

const (
	exampleScope = "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write"
)

type service struct {
	storer      mystore.Store[OAuthSessionSetup]
	vault       myvault.Vault
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
	oauthClient OauthClient
}

func newService(storer mystore.Store[OAuthSessionSetup], vault myvault.Vault, nower mytime.Nower, uuider myuuid.UUIDer, oauthClient OauthClient) *service {
	return &service{
		storer:      storer,
		vault:       vault,
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
	err = s.storer.Put(c, sessionUID, OAuthSessionSetup{
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
	tokenResp := GetTokenResponse{}

	err := s.storer.RunInTransaction(c, func(c context.Context) error {
		session, exist, err := s.storer.Get(c, sessionUID)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("Error fetching session: %s", err))
		}
		if !exist {
			return myerrors.NewNotFoundError(fmt.Errorf("OAuthSessionSetup with uid %s not found", sessionUID))
		}
		returnURL = session.ReturnURL

		// Get token
		tokenResp, err = s.oauthClient.GetAccessToken(c, GetTokenRequest{
			RedirectUri:  session.ReturnURL,
			Code:         code,
			CodeVerifier: session.Verifier,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("Error getting token: %s", err))
		}

		s.logger.Log(c, sessionUID, mylog.SeverityDebug, "token-resp: %+v", tokenResp)

		// Update session
		session.TokenData = &tokenResp
		session.LastModified = func() *time.Time { t := s.nower.Now(); return &t }()
		session.Done = true
		err = s.storer.Put(c, sessionUID, session)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("Error storing session: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Complete oauth session-setup %s", sessionUID)

	// Store token in vault
	err = s.vault.Put(c, myvault.CurrentToken, myvault.Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	})
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("Error storing token in vault: %s", err))
	}

	// TODO Publish that a new token is available

	return returnURL, nil
}

func (s service) refreshToken(c context.Context) error {
	err := s.storer.RunInTransaction(c, func(c context.Context) error {
		currentToken, exists, err := s.vault.Get(c, myvault.CurrentToken)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("Error fetching current token:%s", err))
		}

		if !exists {
			// cannot refreshToken without a token: do not consider this a failure
			return nil
		}

		refreshedTokenResp, err := s.oauthClient.RefreshAccessToken(c, RefreshTokenRequest{
			AccessToken:  currentToken.AccessToken,
			RefreshToken: currentToken.RefreshToken,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("Error refreshing token: %s", err))
		}

		s.logger.Log(c, "", mylog.SeverityDebug, "token-resp: %+v", refreshedTokenResp)

		// TODO Publish that a new accesss token is available

		// Update token
		currentToken.RefreshToken = refreshedTokenResp.RefreshToken
		currentToken.AccessToken = refreshedTokenResp.AccessToken
		currentToken.ExpiresIn = refreshedTokenResp.ExpiresIn
		err = s.vault.Put(c, myvault.CurrentToken, currentToken)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("Error storing token: %s", err))
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.logger.Log(c, "", mylog.SeverityInfo, "Complete oauth session-refresh-token")

	return nil
}
