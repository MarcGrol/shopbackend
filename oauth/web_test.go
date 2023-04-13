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
)

func TestOauth(t *testing.T) {

	t.Run("Start oauth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, nower, uuider, oauthClient := setup(ctrl)

		// given
		oauthClient.EXPECT().ComposeAuthURL(gomock.Any(), gomock.Any()).Return(authURL, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("123")

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

		session, exists, err := storer.Get(ctx, "123")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "123", session.UID)
		assert.Equal(t, "http://localhost:8888/done", session.ReturnURL)
		assert.NotEmpty(t, session.Verifier)
		assert.Equal(t, "2023-02-27T23:58:59", session.CreatedAt.Format("2006-01-02T15:04:05"))

	})

}

func setup(ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[Session], *mytime.MockNower, *myuuid.MockUUIDer, *MockOauthClient) {
	c := context.TODO()
	router := mux.NewRouter()
	storer, _, _ := mystore.New[Session](c)
	nower := mytime.NewMockNower(ctrl)
	uuider := myuuid.NewMockUUIDer(ctrl)
	oauthClient := NewMockOauthClient(ctrl)
	sut := NewService(storer, nower, uuider, oauthClient)
	sut.RegisterEndpoints(c, router)

	return c, router, storer, nower, uuider, oauthClient
}
