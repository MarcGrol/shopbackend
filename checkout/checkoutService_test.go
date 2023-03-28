package checkout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/myqueue"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
)

var (
	sessionResp = checkout.CreateCheckoutSessionResponse{
		AllowedPaymentMethods:  []string{"ideal", "scheme"},
		Amount:                 checkout.Amount{Value: 12300, Currency: "EUR"},
		Channel:                "Web",
		CountryCode:            "nl",
		MerchantAccount:        "MyMerchantAccount",
		MerchantOrderReference: "123",
		Reference:              "123",
		ReturnUrl:              "https:/a.b/c",
		ShopperEmail:           "marc@home.nl",
		ShopperLocale:          "nl-nl",
		ShopperReference:       "marc-123",
		Store:                  "MyStore",
		TelephoneNumber:        "+31612345678",
		Id:                     "456",
		SessionData:            "lalalallalalaallalalalalalal",
	}
	paymentMethodsResp = checkout.PaymentMethodsResponse{
		PaymentMethods:       &[]checkout.PaymentMethod{},
		StoredPaymentMethods: nil,
	}
)

func TestCheckoutService(t *testing.T) {

	t.Run("Create checkout", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, payer, _, nower := setup(ctrl)

		// given
		payer.EXPECT().Sessions(gomock.Any(), gomock.Any()).Return(&sessionResp, nil)
		payer.EXPECT().PaymentMethods(gomock.Any(), gomock.Any()).Return(&paymentMethodsResp, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)

		// when
		request, _ := http.NewRequest(http.MethodPost, "/checkout/123", strings.NewReader(`amount=12300&currency=EUR&returnUrl=http://a.b/c&countryCode=nl&shopper.locale="nl-nl"`))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td>123</td>") // TODO check more

		checkout, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		assert.Equal(t, "123", checkout.BasketUID)
		assert.Equal(t, "456", checkout.ID)
		assert.Equal(t, "lalalallalalaallalalalalalal", checkout.SessionData)
		assert.Equal(t, "http://a.b/c", checkout.OriginalReturnURL)
	})

	t.Run("Handle checkout status redirect", func(t *testing.T) {
		// TODO
	})

	t.Run("Handle checkout status webhook", func(t *testing.T) {
		// TODO
	})
}

func setup(ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[checkoutmodel.CheckoutContext], *MockPayer, *myqueue.MockTaskQueuer, *mytime.MockNower) {
	c := context.TODO()
	storer, _, _ := mystore.New[checkoutmodel.CheckoutContext](c)
	nower := mytime.NewMockNower(ctrl)
	queuer := myqueue.NewMockTaskQueuer(ctrl)
	payer := NewMockPayer(ctrl)

	sut, _ := NewService(Config{}, payer, storer, queuer, nower, mylog.New("checkout"))
	router := mux.NewRouter()
	sut.RegisterEndpoints(c, router)

	return c, router, storer, payer, queuer, nower
}
