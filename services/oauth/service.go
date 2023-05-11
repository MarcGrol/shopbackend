package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcGrol/shopbackend/lib/codeverifier"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
)

type service struct {
	storer      mystore.Store[OAuthSessionSetup]
	vault       myvault.VaultReadWriter
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
	oauthClient oauthclient.OauthClient
	publisher   mypublisher.Publisher
	providers   providers.OAuthProvider
}

func newService(storer mystore.Store[OAuthSessionSetup], vault myvault.VaultReadWriter, nower mytime.Nower, uuider myuuid.UUIDer, oauthClient oauthclient.OauthClient, pub mypublisher.Publisher, providers providers.OAuthProvider) *service {
	return &service{
		storer:      storer,
		vault:       vault,
		nower:       nower,
		uuider:      uuider,
		oauthClient: oauthClient,
		logger:      mylog.New("oauth"),
		publisher:   pub,
		providers:   providers,
	}
}

func (s *service) CreateTopics(c context.Context) error {
	err := s.publisher.CreateTopic(c, oauthevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", oauthevents.TopicName, err)
	}

	return nil
}

func (s *service) getOauthStatus(c context.Context) (map[string]OAuthStatus, error) {

	s.logger.Log(c, "", mylog.SeverityInfo, "Get oauth status")

	statuses := map[string]OAuthStatus{}
	for name := range s.providers.All() {
		tokenUID := CreateTokenUID(name)
		token, exists, err := s.vault.Get(c, tokenUID)
		if err != nil {
			return statuses, myerrors.NewInternalError(err)
		}

		statuses[name] = tokenToStatus(token, exists)
	}

	return statuses, nil
}

func tokenToStatus(token myvault.Token, exists bool) OAuthStatus {
	return OAuthStatus{
		ProviderName: token.ProviderName,
		ClientID:     token.ClientID,
		SessionUID:   token.SessionUID,
		Scopes:       token.Scopes,
		CreatedAt:    token.CreatedAt,
		LastModified: token.LastModified,
		Status:       exists && token.AccessToken != "",
		ValidUntil: func() *time.Time {
			if token.ExpiresIn == 0 {
				return nil
			} else if token.LastModified != nil {
				t := token.LastModified.Add(time.Second * time.Duration(token.ExpiresIn))
				return &t
			}
			t := token.CreatedAt.Add(time.Second * time.Duration(token.ExpiresIn))
			return &t
		}(),
	}
}

func (s *service) start(c context.Context, providerName string, requestedScopes string, originalReturnURL string, currentHostname string) (string, error) {
	now := s.nower.Now()
	sessionUID := s.uuider.Create()

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Start oauth session-setup %s", sessionUID)

	provider, err := s.providers.Get(providerName)
	if err != nil {
		return "", myerrors.NewInvalidInputError(fmt.Errorf("provider with name '%s' not known", providerName))
	}
	if requestedScopes == "" {
		requestedScopes = provider.DefaultScopes
	}

	codeVerifier, err := codeverifier.NewVerifier()
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error creating verifier: %s", err))
	}
	codeVerifierValue := codeVerifier.GetValue()

	authURL := ""
	err = s.storer.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		authURL, err = s.oauthClient.ComposeAuthURL(c, oauthclient.ComposeAuthURLRequest{
			ProviderName:  providerName,
			CompletionURL: createCompletionURL(currentHostname), // Be called back here when authorisation has completed
			Scope:         requestedScopes,
			State:         sessionUID,
			CodeVerifier:  codeVerifierValue,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error composing auth url: %s", err))
		}

		// Create new session
		err := s.storer.Put(c, sessionUID, OAuthSessionSetup{
			UID:          sessionUID,
			ProviderName: providerName,
			ClientID:     provider.ClientID,
			Scopes:       requestedScopes,
			ReturnURL:    originalReturnURL,
			Verifier:     codeVerifierValue,
			CreatedAt:    now,
			LastModified: &now,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing: %s", err))
		}

		err = s.publisher.Publish(c, oauthevents.TopicName, oauthevents.OAuthSessionSetupStarted{
			ProviderName: providerName,
			ClientID:     provider.ClientID,
			SessionUID:   sessionUID,
			Scopes:       requestedScopes,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Completed first step of oauth session-setup %s", sessionUID)

	return authURL, nil
}

func (s *service) done(c context.Context, sessionUID string, code string, currentHostname string) (string, error) {
	now := s.nower.Now()

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Continue with oauth session-setup (create-token) %s", sessionUID)

	returnURL := ""
	tokenResp := oauthclient.GetTokenResponse{}
	err := s.storer.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		session, exist, err := s.storer.Get(c, sessionUID)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error fetching session: %s", err))
		}
		if !exist {
			return myerrors.NewNotFoundError(fmt.Errorf("session with uid %s not found", sessionUID))
		}
		returnURL = session.ReturnURL

		// Get token
		tokenResp, err = s.oauthClient.GetAccessToken(c, oauthclient.GetTokenRequest{
			ProviderName: session.ProviderName,
			RedirectUri:  createCompletionURL(currentHostname),
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

		// Store new token in vault
		err = s.vault.Put(c, CreateTokenUID(session.ProviderName), myvault.Token{
			ProviderName: session.ProviderName,
			ClientID:     session.ClientID,
			SessionUID:   session.UID,
			Scopes:       session.Scopes,
			CreatedAt:    session.CreatedAt,
			LastModified: session.LastModified,
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			ExpiresIn:    tokenResp.ExpiresIn,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing token in vault: %s", err))
		}

		err = s.publisher.Publish(c, oauthevents.TopicName, oauthevents.OAuthSessionSetupCompleted{
			ProviderName: session.ProviderName,
			ClientID:     session.ClientID,
			SessionUID:   sessionUID,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	s.logger.Log(c, sessionUID, mylog.SeverityInfo, "Completed oauth session-setup (token-created) %s", sessionUID)

	return returnURL, nil
}

func createCompletionURL(hostname string) string {
	return fmt.Sprintf("%s/oauth/done", hostname)
}

func (s *service) refreshToken(c context.Context, providerName string) (myvault.Token, error) {
	now := s.nower.Now()
	uid := s.uuider.Create()

	s.logger.Log(c, "", mylog.SeverityInfo, "Start oauth token-refresh")

	newToken := myvault.Token{}
	err := s.storer.RunInTransaction(c, func(c context.Context) error {
		tokenUID := CreateTokenUID(providerName)
		currentToken, exists, err := s.vault.Get(c, tokenUID)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error fetching token %s:%s", tokenUID, err))
		}

		if !exists || currentToken.RefreshToken == "" {
			s.logger.Log(c, "", mylog.SeverityInfo, "Cannot refresh token: no token to")
			// Do not consider this a failure
			return nil
		}

		newTokenResp, err := s.oauthClient.RefreshAccessToken(c, oauthclient.RefreshTokenRequest{
			ProviderName: currentToken.ProviderName,
			RefreshToken: currentToken.RefreshToken,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error refreshing token: %s", err))
		}

		s.logger.Log(c, "", mylog.SeverityDebug, "refresh-token-resp: %+v", newTokenResp)

		newToken = myvault.Token{
			ProviderName: currentToken.ProviderName,
			ClientID:     currentToken.ClientID,
			SessionUID:   currentToken.SessionUID,
			Scopes:       currentToken.Scopes,
			CreatedAt:    currentToken.CreatedAt,
			LastModified: &now,
			AccessToken:  newTokenResp.AccessToken,
			RefreshToken: newTokenResp.RefreshToken,
			ExpiresIn:    newTokenResp.ExpiresIn,
		}
		// Update token
		err = s.vault.Put(c, CreateTokenUID(currentToken.ProviderName), newToken)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing token: %s", err))
		}

		err = s.publisher.Publish(c, oauthevents.TopicName, oauthevents.OAuthTokenRefreshCompleted{
			ProviderName: currentToken.ProviderName,
			UID:          uid,
			ClientID:     currentToken.ClientID,
			SessionUID:   currentToken.SessionUID,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return newToken, err
	}

	s.logger.Log(c, "", mylog.SeverityInfo, "Completed oauth token-refresh")

	return newToken, nil
}

func (s *service) cancelToken(c context.Context, providerName string) error {
	now := s.nower.Now()

	s.logger.Log(c, "", mylog.SeverityInfo, "Start canceling token-refresh")

	newToken := myvault.Token{}
	err := s.storer.RunInTransaction(c, func(c context.Context) error {
		tokenUID := CreateTokenUID(providerName)
		currentToken, exists, err := s.vault.Get(c, tokenUID)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error fetching token %s:%s", tokenUID, err))
		}

		if !exists || currentToken.RefreshToken == "" {
			s.logger.Log(c, "", mylog.SeverityInfo, "Cannot cancel token: no token to")
			// Do not consider this a failure
			return nil
		}

		err = s.oauthClient.CancelAccessToken(c, oauthclient.CancelTokenRequest{
			ProviderName: currentToken.ProviderName,
			AccessToken:  currentToken.AccessToken,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error canceling token: %s", err))
		}

		newToken = myvault.Token{
			ProviderName: currentToken.ProviderName,
			ClientID:     currentToken.ClientID,
			SessionUID:   "",
			Scopes:       "",
			CreatedAt:    currentToken.CreatedAt,
			LastModified: &now,
			AccessToken:  "",
			RefreshToken: "",
			ExpiresIn:    0,
		}
		// Update token
		err = s.vault.Put(c, CreateTokenUID(currentToken.ProviderName), newToken)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing token: %s", err))
		}

		err = s.publisher.Publish(c, oauthevents.TopicName, oauthevents.OAuthTokenCancelCompleted{
			ProviderName: currentToken.ProviderName,
			ClientID:     currentToken.ClientID,
			SessionUID:   currentToken.SessionUID,
		})
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error publishing event: %s", err))
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.logger.Log(c, "", mylog.SeverityInfo, "Completed oauth token-cancelation")

	return nil
}

func CreateTokenUID(providerName string) string {
	return myvault.CurrentToken + "_" + providerName
}
