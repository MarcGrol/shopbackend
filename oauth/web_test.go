package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
)

func TestOauth(t *testing.T) {

	t.Run("Start oauth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, nower, uuider, oauthClient := setup(ctrl)

		// given
		oauthClient.EXPECT().ComposeAuthURL(gomock.Any(), gomock.Any()).Return(authURL, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("abcdef")

		// when
		request, err := http.NewRequest(http.MethodGet, "/oauth/start?returnURL=http://localhost:8888/done", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, authURL, redirectURL)

		session, exists, err := storer.Get(ctx, "abcdef")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "abcdef", session.UID)
		assert.Equal(t, "http://localhost:8888/done", session.ReturnURL)
		assert.NotEmpty(t, session.Verifier)
		assert.Equal(t, "2023-02-27T23:58:59", session.CreatedAt.Format("2006-01-02T15:04:05"))
		assert.Nil(t, session.LastModified)

	})

	t.Run("Done oauth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, vault, nower, _, oauthClient := setup(ctrl)

		exampleResp := GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    12345,
			AccessToken:  "abc123",
			Scope:        exampleScope,
			RefreshToken: "rst456",
		}

		// given
		storer.Put(ctx, "abcdef", OAuthSessionSetup{
			UID:       "abcdef",
			ReturnURL: "http://localhost:8888/basket",
			Verifier:  "exampleHash",
			CreatedAt: mytime.ExampleTime,
			TokenData: &exampleResp,
		})
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		oauthClient.EXPECT().GetAccessToken(gomock.Any(), gomock.Any()).Return(exampleResp, nil)

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

		token, exists, err := vault.Get(ctx, myvault.CurrentToken)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "abc123", token.AccessToken)
		assert.Equal(t, "rst456", session.TokenData.RefreshToken)
	})

}

func setup(ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[OAuthSessionSetup], myvault.Vault, *mytime.MockNower, *myuuid.MockUUIDer, *MockOauthClient) {
	c := context.TODO()
	router := mux.NewRouter()
	storer, _, _ := mystore.New[OAuthSessionSetup](c)
	vault, _, _ := myvault.New(c)
	nower := mytime.NewMockNower(ctrl)
	uuider := myuuid.NewMockUUIDer(ctrl)
	oauthClient := NewMockOauthClient(ctrl)
	sut := NewService(storer, vault, nower, uuider, oauthClient)
	sut.RegisterEndpoints(c, router)

	return c, router, storer, vault, nower, uuider, oauthClient
}
