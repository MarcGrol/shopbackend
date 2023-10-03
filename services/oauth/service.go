package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthvault"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
)

type service struct {
	partyVault   myvault.VaultReadWriter[providers.OauthParty]
	sessionStore mystore.Store[OAuthSessionSetup]
	vault        myvault.VaultReadWriter[oauthvault.Token]
	nower        mytime.Nower
	uuider       myuuid.UUIDer
	logger       mylog.Logger
	oauthClient  oauthclient.OauthClient
	publisher    mypublisher.Publisher
	providers    providers.OAuthProvider
}

func newService(partyVault myvault.VaultReadWriter[providers.OauthParty], sessionStore mystore.Store[OAuthSessionSetup], vault myvault.VaultReadWriter[oauthvault.Token], nower mytime.Nower, uuider myuuid.UUIDer, oauthClient oauthclient.OauthClient, pub mypublisher.Publisher, providers providers.OAuthProvider) *service {
	return &service{
		partyVault:   partyVault,
		sessionStore: sessionStore,
		vault:        vault,
		nower:        nower,
		uuider:       uuider,
		oauthClient:  oauthClient,
		logger:       mylog.New("oauth"),
		publisher:    pub,
		providers:    providers,
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

func tokenToStatus(token oauthvault.Token, exists bool) OAuthStatus {
	return OAuthStatus{
		ProviderName: token.ProviderName,
		ClientID:     token.ClientID,
		SessionUID:   token.SessionUID,
		Scopes:       token.Scopes,
		CreatedAt:    token.CreatedAt,
		LastModified: token.LastModified,
		Status:       exists && token.AccessToken != "",
		ValidUntil:   token.ExpiresIn,
	}
}

func (s *service) start(c context.Context, providerName string, clientID string, clientSecret string, requestedScopes string, originalReturnURL string, currentHostname string) (string, error) {
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

	authURL, codeVerifierValue, err := s.oauthClient.ComposeAuthURL(c, oauthclient.ComposeAuthURLRequest{
		ProviderName:  providerName,
		ClientID:      clientID,
		CompletionURL: createCompletionURL(currentHostname), // Be called back here when authorisation has completed
		Scope:         requestedScopes,
		State:         sessionUID,
	})
	if err != nil {
		return "", myerrors.NewInternalError(fmt.Errorf("error composing auth url: %s", err))
	}

	err = s.sessionStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		err = s.partyVault.Put(c, providerName, provider)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing party details: %s", err))
		}

		// Create new session
		err := s.sessionStore.Put(c, sessionUID, OAuthSessionSetup{
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
	err := s.sessionStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		session, exist, err := s.sessionStore.Get(c, sessionUID)
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
			RedirectURI:  createCompletionURL(currentHostname),
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
		err = s.sessionStore.Put(c, sessionUID, session)
		if err != nil {
			return myerrors.NewInternalError(fmt.Errorf("error storing session: %s", err))
		}

		// Store new token in vault
		err = s.vault.Put(c, CreateTokenUID(session.ProviderName), oauthvault.Token{
			ProviderName: session.ProviderName,
			ClientID:     session.ClientID,
			SessionUID:   session.UID,
			Scopes:       session.Scopes,
			CreatedAt:    session.CreatedAt,
			LastModified: session.LastModified,
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			ExpiresIn:    calculateExpriesIn(session.CreatedAt, tokenResp.ExpiresIn),
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

func (s *service) refreshToken(c context.Context, providerName string) (oauthvault.Token, error) {
	now := s.nower.Now()
	uid := s.uuider.Create()

	s.logger.Log(c, "", mylog.SeverityInfo, "Start oauth token-refresh")

	newToken := oauthvault.Token{}
	err := s.sessionStore.RunInTransaction(c, func(c context.Context) error {
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

		newToken = oauthvault.Token{
			ProviderName: currentToken.ProviderName,
			ClientID:     currentToken.ClientID,
			SessionUID:   currentToken.SessionUID,
			Scopes:       currentToken.Scopes,
			CreatedAt:    currentToken.CreatedAt,
			LastModified: &now,
			AccessToken:  newTokenResp.AccessToken,
			RefreshToken: newTokenResp.RefreshToken,
			ExpiresIn:    calculateExpriesIn(now, newTokenResp.ExpiresIn),
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

func calculateExpriesIn(lastModified time.Time, expiresIn int) *time.Time {
	if expiresIn == 0 {
		return nil
	}
	t := lastModified.Add(time.Second * time.Duration(expiresIn))
	return &t
}

func (s *service) cancelToken(c context.Context, providerName string) error {
	now := s.nower.Now()

	s.logger.Log(c, "", mylog.SeverityInfo, "Start canceling token-refresh")

	newToken := oauthvault.Token{}
	err := s.sessionStore.RunInTransaction(c, func(c context.Context) error {
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

		newToken = oauthvault.Token{
			ProviderName: currentToken.ProviderName,
			ClientID:     currentToken.ClientID,
			SessionUID:   "",
			Scopes:       "",
			CreatedAt:    currentToken.CreatedAt,
			LastModified: &now,
			AccessToken:  "",
			RefreshToken: "",
			ExpiresIn:    nil,
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
	return oauthvault.CurrentToken + "_" + providerName
}
