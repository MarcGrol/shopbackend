package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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

const (
	adyenExampleScopes = "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write"
)

func TestOauth(t *testing.T) {

	t.Run("Get oauth admin page", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, _, tokenVault, _, _, _, _ := setup(t, ctrl)

		tokenVault.EXPECT().Get(gomock.Any(), CreateTokenUID("adyen")).Return(oauthvault.Token{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "xyz",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "abc123",
			RefreshToken: "rst456",
			ExpiresIn:    func() *time.Time { t := mytime.ExampleTime.Add(24 * 60 * 60 * time.Second); return &t }(),
		}, true, nil)

		tokenVault.EXPECT().Get(gomock.Any(), CreateTokenUID("stripe")).Return(oauthvault.Token{
			ProviderName: "stripe",
			ClientID:     "stripe_client_id",
			SessionUID:   "",
			Scopes:       "",
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "",
			RefreshToken: "",
			ExpiresIn:    nil,
		}, true, nil)

		tokenVault.EXPECT().Get(gomock.Any(), CreateTokenUID("mollie")).Return(oauthvault.Token{
			ProviderName: "mollie",
			ClientID:     "mollie_client_id",
			SessionUID:   "",
			Scopes:       "",
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "",
			RefreshToken: "",
			ExpiresIn:    nil,
		}, true, nil)

		// when
		request, err := http.NewRequest(http.MethodGet, "/oauth/admin", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()

		assert.Contains(t, got, "<td>adyen_client_id</td>")
		assert.Contains(t, got, "Refresh adyen token")
		assert.Contains(t, got, "Invalidate adyen token")
		assert.NotContains(t, got, "OAuth connect with adyen")

		assert.NotContains(t, got, "Refresh stripe token")
		assert.NotContains(t, got, "Invalidate stripe token")
		assert.Contains(t, got, "OAuth connect with stripe")
	})

	t.Run("Start oauth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, partyVault, sessionStorer, _, nower, uuider, oauthClient, publisher := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("abcdef")

		sessionStorer.EXPECT().RunInTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, f func(ctx context.Context) error) error {
				return f(ctx)
			})
		partyVault.EXPECT().Put(gomock.Any(), "adyen", gomock.Any()).Return(nil)
		sessionStorer.EXPECT().Put(gomock.Any(), "abcdef", gomock.Any()).DoAndReturn(
			func(ctx context.Context, uid string, session OAuthSessionSetup) error {
				assert.Equal(t, "abcdef", session.UID)
				assert.Equal(t, "http://localhost:8888/basket", session.ReturnURL)
				assert.NotEmpty(t, session.Verifier)
				assert.Equal(t, "2023-02-27T23:58:59", session.CreatedAt.Format("2006-01-02T15:04:05"))
				assert.Equal(t, "2023-02-27T23:58:59", session.LastModified.Format("2006-01-02T15:04:05"))
				return nil
			})

		publisher.EXPECT().Publish(gomock.Any(), oauthevents.TopicName, oauthevents.OAuthSessionSetupStarted{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "abcdef",
			Scopes:       "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write",
		}).Return(nil)
		oauthClient.EXPECT().ComposeAuthURL(gomock.Any(), gomock.Any()).Return("http://my_url.com", "mychallenge", nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/oauth/start/adyen",
			strings.NewReader(`clientID=abc&clientSecret=xyz&returnURL=http://localhost:8888/basket&scopes=psp.onlinepayment:write psp.accountsettings:write psp.webhook:write`))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "http://my_url.com", redirectURL)

	})

	t.Run("Done oauth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, sessionStorer, tokenVault, nower, _, oauthClient, publisher := setup(t, ctrl)

		exampleResp := oauthclient.GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    24 * 60 * 60,
			AccessToken:  "abc123",
			Scope:        adyenExampleScopes,
			RefreshToken: "rst456",
		}

		// given
		sessionStorer.EXPECT().RunInTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, f func(ctx context.Context) error) error {
				return f(ctx)
			})
		sessionStorer.EXPECT().Get(gomock.Any(), "abcdef").Return(OAuthSessionSetup{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			UID:          "abcdef",
			Scopes:       adyenExampleScopes,
			ReturnURL:    "http://localhost:8888/basket",
			Verifier:     "exampleHash",
			CreatedAt:    mytime.ExampleTime,
			TokenData:    &exampleResp,
		}, true, nil)
		sessionStorer.EXPECT().Put(gomock.Any(), "abcdef", gomock.Any()).DoAndReturn(
			func(ctx context.Context, uid string, session OAuthSessionSetup) error {
				assert.Equal(t, "abcdef", session.UID)
				assert.Equal(t, "http://localhost:8888/basket", session.ReturnURL)
				assert.NotEmpty(t, session.Verifier)
				assert.Equal(t, "2023-02-27T23:58:59", session.CreatedAt.Format("2006-01-02T15:04:05"))
				assert.Equal(t, "2023-02-27T23:58:59", session.LastModified.Format("2006-01-02T15:04:05"))
				return nil
			})
		oauthClient.EXPECT().GetAccessToken(gomock.Any(), oauthclient.GetTokenRequest{
			ProviderName: "adyen",
			RedirectURI:  "http://localhost:8888/oauth/done",
			Code:         "789",
			CodeVerifier: "exampleHash",
		}).Return(exampleResp, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		tokenVault.EXPECT().Put(gomock.Any(), CreateTokenUID("adyen"), oauthvault.Token{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "abcdef",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "abc123",
			RefreshToken: "rst456",
			ExpiresIn:    func() *time.Time { t := mytime.ExampleTime.Add(24 * 60 * 60 * time.Second); return &t }(),
		}).Return(nil)
		publisher.EXPECT().Publish(gomock.Any(), oauthevents.TopicName, oauthevents.OAuthSessionSetupCompleted{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "abcdef",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodGet, "/oauth/done?code=789&state=abcdef", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "http://localhost:8888/basket", redirectURL)

	})
	t.Run("Refresh token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, sessionStorer, vault, nower, uuider, oauthClient, publisher := setup(t, ctrl)

		// given
		sessionStorer.EXPECT().RunInTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, f func(ctx context.Context) error) error {
				return f(ctx)
			})

		vault.EXPECT().Get(gomock.Any(), CreateTokenUID("adyen")).Return(oauthvault.Token{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "xyz",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "abc123",
			RefreshToken: "rst456",
			ExpiresIn:    func() *time.Time { t := mytime.ExampleTime.Add(24 * 60 * 60 * time.Second); return &t }(),
		}, true, nil)
		oauthClient.EXPECT().RefreshAccessToken(gomock.Any(), oauthclient.RefreshTokenRequest{
			ProviderName: "adyen",
			RefreshToken: "rst456",
		}).Return(oauthclient.GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    24 * 60 * 60,
			AccessToken:  "abc123new",
			Scope:        adyenExampleScopes,
			RefreshToken: "rst456new",
		}, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("xyz")
		vault.EXPECT().Put(gomock.Any(), CreateTokenUID("adyen"), oauthvault.Token{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "xyz",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "abc123new",
			RefreshToken: "rst456new",
			ExpiresIn:    func() *time.Time { t := mytime.ExampleTime.Add(24 * 60 * 60 * time.Second); return &t }(),
		}).Return(nil)
		publisher.EXPECT().Publish(gomock.Any(), oauthevents.TopicName, oauthevents.OAuthTokenRefreshCompleted{
			ProviderName: "adyen",
			UID:          "xyz",
			ClientID:     "adyen_client_id",
			SessionUID:   "xyz",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/oauth/refresh/adyen", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		assert.Equal(t, "/oauth/admin", response.Header().Get("Location"))
	})

	t.Run("Cancel token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, sessionStorer, vault, nower, _, oauthClient, publisher := setup(t, ctrl)

		// given
		sessionStorer.EXPECT().RunInTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, f func(ctx context.Context) error) error {
				return f(ctx)
			})
		vault.EXPECT().Get(gomock.Any(), CreateTokenUID("adyen")).Return(oauthvault.Token{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "xyz",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "abc123",
			RefreshToken: "rst456",
			ExpiresIn:    func() *time.Time { t := mytime.ExampleTime.Add(24 * 60 * 60 * time.Second); return &t }(),
		}, true, nil)
		oauthClient.EXPECT().CancelAccessToken(gomock.Any(), oauthclient.CancelTokenRequest{
			ProviderName: "adyen",
			AccessToken:  "abc123",
		}).Return(nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		vault.EXPECT().Put(gomock.Any(), CreateTokenUID("adyen"), oauthvault.Token{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "",
			Scopes:       "",
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "",
			RefreshToken: "",
			ExpiresIn:    nil,
		}).Return(nil)
		publisher.EXPECT().Publish(gomock.Any(), oauthevents.TopicName, oauthevents.OAuthTokenCancelCompleted{
			ProviderName: "adyen",
			ClientID:     "adyen_client_id",
			SessionUID:   "xyz",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/oauth/cancel/adyen", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		assert.Equal(t, "/oauth/admin", response.Header().Get("Location"))
	})
}

func setup(t *testing.T, ctrl *gomock.Controller) (context.Context, *mux.Router, *myvault.MockVaultReadWriter[providers.OauthParty], *mystore.MockStore[OAuthSessionSetup], *myvault.MockVaultReadWriter[oauthvault.Token], *mytime.MockNower, *myuuid.MockUUIDer, *oauthclient.MockOauthClient, *mypublisher.MockPublisher) {
	ctx := context.TODO()
	router := mux.NewRouter()
	partyVault := myvault.NewMockVaultReadWriter[providers.OauthParty](ctrl)
	sessionStore := mystore.NewMockStore[OAuthSessionSetup](ctrl)
	tokenVault := myvault.NewMockVaultReadWriter[oauthvault.Token](ctrl)
	nower := mytime.NewMockNower(ctrl)
	uuider := myuuid.NewMockUUIDer(ctrl)
	oauthClient := oauthclient.NewMockOauthClient(ctrl)
	publisher := mypublisher.NewMockPublisher(ctrl)
	sut := NewService(partyVault, sessionStore, tokenVault, nower, uuider, oauthClient, publisher, providers.NewProviders())

	publisher.EXPECT().CreateTopic(gomock.Any(), oauthevents.TopicName).Return(nil)

	err := sut.RegisterEndpoints(ctx, router)
	assert.NoError(t, err)

	return ctx, router, partyVault, sessionStore, tokenVault, nower, uuider, oauthClient, publisher
}
