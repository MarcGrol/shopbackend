package checkout

import (
	"context"
	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
)

func TestCheckoutService(t *testing.T) {

	t.Run("Create checkout", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, nower := setup(ctrl)
		nower.EXPECT().Now().Return(mytime.ExampleTime)

		// given

		// when
		request, _ := http.NewRequest(http.MethodPost, "/checkout/123", strings.NewReader(`amount=12300&currency=EUR&returnUrl=abc&countryCode=nl&shopper.locale="nl-nl"`))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td>123</td>") // TODO check more

		_, found, _ := storer.Get(ctx, "123")
		assert.True(t, found)
	})

	t.Run("Handle checkout status redirect", func(t *testing.T) {
		// TODO
	})

	t.Run("Handle checkout status webhook", func(t *testing.T) {
		// TODO
	})
}

func setup(ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[checkoutmodel.CheckoutContext], *myqueue.MockTaskQueuer, *mytime.MockNower) {
	c := context.TODO()
	storer, _, _ := mystore.New[checkoutmodel.CheckoutContext](c)
	nower := mytime.NewMockNower(ctrl)
	queuer := myqueue.NewMockTaskQueuer(ctrl)

	sut, _ := NewService(nil, storer, queuer, nower, mylog.New("checkout"))
	router := mux.NewRouter()
	sut.RegisterEndpoints(c, router)

	return c, router, storer, queuer, nower
}
