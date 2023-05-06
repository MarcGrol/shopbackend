package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
)

func TestOauth(t *testing.T) {

	t.Run("Start oauth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, nower, uuider, oauthClient, publisher := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("abcdef")
		publisher.EXPECT().Publish(gomock.Any(), oauthevents.TopicName, oauthevents.OAuthSessionSetupStarted{
			ProviderName: "adyen",
			SessionUID:   "abcdef",
			ClientID:     "client12345",
			Scopes:       "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write",
		}).Return(nil)
		oauthClient.EXPECT().ComposeAuthURL(gomock.Any(), gomock.Any()).Return("http://my_url.com", nil)

		// when
		request, err := http.NewRequest(http.MethodGet, "/oauth/start/adyen?returnURL=http://localhost:8888/basket&scopes=psp.onlinepayment:write psp.accountsettings:write psp.webhook:write", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "http://my_url.com", redirectURL)

		session, exists, err := storer.Get(ctx, "abcdef")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "abcdef", session.UID)
		assert.Equal(t, "http://localhost:8888/basket", session.ReturnURL)
		assert.NotEmpty(t, session.Verifier)
		assert.Equal(t, "2023-02-27T23:58:59", session.CreatedAt.Format("2006-01-02T15:04:05"))
		assert.Equal(t, "2023-02-27T23:58:59", session.LastModified.Format("2006-01-02T15:04:05"))
	})

	t.Run("Done oauth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, vault, nower, _, oauthClient, publisher := setup(t, ctrl)

		exampleResp := GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    12345,
			AccessToken:  "abc123",
			Scope:        adyenExampleScopes,
			RefreshToken: "rst456",
		}

		// given
		storer.Put(ctx, "abcdef", OAuthSessionSetup{
			ProviderName: "adyen",
			ClientID:     "client12345",
			UID:          "abcdef",
			Scopes:       adyenExampleScopes,
			ReturnURL:    "http://localhost:8888/basket",
			Verifier:     "exampleHash",
			CreatedAt:    mytime.ExampleTime,
			TokenData:    &exampleResp,
		})
		oauthClient.EXPECT().GetAccessToken(gomock.Any(), GetTokenRequest{
			ProviderName: "adyen",
			RedirectUri:  "http://localhost:8888/oauth/done",
			Code:         "789",
			CodeVerifier: "exampleHash",
		}).Return(exampleResp, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		vault.EXPECT().Put(gomock.Any(), myvault.CurrentToken, myvault.Token{
			ProviderName: "adyen",
			ClientID:     "client12345",
			SessionUID:   "abcdef",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "abc123",
			RefreshToken: "rst456",
			ExpiresIn:    12345,
		}).Return(nil)
		publisher.EXPECT().Publish(gomock.Any(), oauthevents.TopicName, oauthevents.OAuthSessionSetupCompleted{
			SessionUID: "abcdef",
			Success:    true,
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

		session, exists, err := storer.Get(ctx, "abcdef")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "abcdef", session.UID)
		assert.Equal(t, "abc123", session.TokenData.AccessToken)
		assert.Equal(t, "2023-02-27T23:58:59", session.LastModified.Format("2006-01-02T15:04:05"))
		assert.True(t, session.Done)

	})

	t.Run("Refresh token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, vault, nower, uuider, oauthClient, publisher := setup(t, ctrl)

		// given
		storer.Put(ctx, "abcdef", OAuthSessionSetup{
			UID:          "abcdef",
			ProviderName: "adyen",
			ClientID:     "client12345",
			Scopes:       adyenExampleScopes,
			ReturnURL:    "http://localhost:8888/basket",
			Verifier:     "exampleHash",
			CreatedAt:    mytime.ExampleTime,
			TokenData: &GetTokenResponse{
				TokenType:    "bearer",
				ExpiresIn:    12345,
				AccessToken:  "abc123",
				Scope:        adyenExampleScopes,
				RefreshToken: "rst456",
			},
		})
		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken).Return(myvault.Token{
			ProviderName: "adyen",
			ClientID:     "client12345",
			SessionUID:   "xyz",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			AccessToken:  "abc123",
			RefreshToken: "rst456",
			ExpiresIn:    12345,
		}, true, nil)
		oauthClient.EXPECT().RefreshAccessToken(gomock.Any(), RefreshTokenRequest{
			ProviderName: "adyen",
			RefreshToken: "rst456",
		}).Return(GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    123456,
			AccessToken:  "abc123new",
			Scope:        adyenExampleScopes,
			RefreshToken: "rst456new",
		}, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("xyz")
		vault.EXPECT().Put(gomock.Any(), myvault.CurrentToken, myvault.Token{
			ProviderName: "adyen",
			ClientID:     "client12345",
			SessionUID:   "xyz",
			Scopes:       adyenExampleScopes,
			CreatedAt:    mytime.ExampleTime,
			LastModified: &mytime.ExampleTime,
			AccessToken:  "abc123new",
			RefreshToken: "rst456new",
			ExpiresIn:    123456,
		}).Return(nil)
		publisher.EXPECT().Publish(gomock.Any(), oauthevents.TopicName, oauthevents.OAuthTokenRefreshCompleted{
			ProviderName: "adyen",
			UID:          "xyz",
			ClientID:     "client12345",
			Success:      true,
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodGet, "/oauth/refresh", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
	})

}

func setup(t *testing.T, ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[OAuthSessionSetup], *myvault.MockVaultReadWriter, *mytime.MockNower, *myuuid.MockUUIDer, *MockOauthClient, *mypublisher.MockPublisher) {
	c := context.TODO()
	router := mux.NewRouter()
	storer, _, _ := mystore.New[OAuthSessionSetup](c)
	vault := myvault.NewMockVaultReadWriter(ctrl)
	nower := mytime.NewMockNower(ctrl)
	uuider := myuuid.NewMockUUIDer(ctrl)
	oauthClient := NewMockOauthClient(ctrl)
	publisher := mypublisher.NewMockPublisher(ctrl)
	sut := NewService("client12345", storer, vault, nower, uuider, oauthClient, publisher)

	publisher.EXPECT().CreateTopic(c, oauthevents.TopicName).Return(nil)

	err := sut.RegisterEndpoints(c, router)
	assert.NoError(t, err)

	return c, router, storer, vault, nower, uuider, oauthClient, publisher
}
