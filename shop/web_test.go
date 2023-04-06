package shop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

var (
	basket1 = shopmodel.Basket{UID: "123", CreatedAt: time.Now(), TotalPrice: 100, Currency: "EUR", InitialPaymentStatus: "success", FinalPaymentStatus: ""}
	basket2 = shopmodel.Basket{UID: "456", CreatedAt: time.Now().Add(time.Minute), TotalPrice: 200, Currency: "EUR", InitialPaymentStatus: "success", FinalPaymentStatus: "AUTHORISATION=true"}
	baskets = []shopmodel.Basket{basket1, basket2}
)

func TestBasketService(t *testing.T) {

	t.Run("List baskets", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, _ := setup(ctrl)

		// given
		storer.Put(ctx, basket1.UID, basket1)
		storer.Put(ctx, basket2.UID, basket2)

		// when
		request, err := http.NewRequest(http.MethodGet, "/basket", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td><a href=\"/basket/123\">123</a></td>")
		assert.Contains(t, got, "<td><a href=\"/basket/456\">456</a></td>")
	})

	t.Run("Get basket", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// given
		ctx, router, storer, _, _ := setup(ctrl)

		// given
		storer.Put(ctx, basket1.UID, basket1)

		// when
		request, err := http.NewRequest(http.MethodGet, "/basket/123", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td>123</td>")
	})

	t.Run("Get basket not exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// given
		_, router, _, _, _ := setup(ctrl)

		// when
		request, err := http.NewRequest(http.MethodGet, "/basket/123", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 404, response.Code)
	})

	t.Run("Put basket", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, nower, uuider := setup(ctrl)

		// given
		storer.Put(ctx, basket1.UID, basket1)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("123")

		// when
		request, err := http.NewRequest(http.MethodPost, "/basket", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "http://localhost:8888/basket/"+basket1.UID, redirectURL)
		basket, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		assert.Equal(t, "123", basket.UID)
		assert.Equal(t, int64(51000), basket.TotalPrice)
		assert.Equal(t, "EUR", basket.Currency)

	})

	t.Run("Handle status redirect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, nower, _ := setup(ctrl)

		// given
		storer.Put(ctx, basket1.UID, basket1)
		nower.EXPECT().Now().Return(mytime.ExampleTime)

		// when
		request, err := http.NewRequest(http.MethodGet, "/basket/123/checkout/completed", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
	})

	t.Run("Handle async update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, nower, _ := setup(ctrl)

		// given
		storer.Put(ctx, basket1.UID, basket1)
		nower.EXPECT().Now().Return(mytime.ExampleTime)

		// when
		request, err := http.NewRequest(http.MethodPut, "/api/basket/123/status/AUTHORISATION/true", strings.NewReader(`{
   "live":"false",
   "notificationItems":[
      {
         "NotificationRequestItem":{
            "eventCode":"AUTHORISATION",
            "success":"true",
            "eventDate":"2019-06-28T18:03:50+01:00",
            "merchantAccountCode":"YOUR_MERCHANT_ACCOUNT",
            "pspReference": "7914073381342284",
            "merchantReference": "456",
            "amount": {
                "value":200,
                "currency":"EUR"
            }
         }
      }
   ]
}`))
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "{}")
	})
}

func setup(ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[shopmodel.Basket], *mytime.MockNower, *myuuid.MockUUIDer) {
	c := context.TODO()
	storer, _, _ := mystore.New[shopmodel.Basket](c)
	nower := mytime.NewMockNower(ctrl)
	uuider := myuuid.NewMockUUIDer(ctrl)
	sut := NewService(storer, nower, uuider, mylog.New("basket"))
	router := mux.NewRouter()
	sut.RegisterEndpoints(c, router)

	return c, router, storer, nower, uuider
}
