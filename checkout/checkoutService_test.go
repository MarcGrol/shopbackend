package checkout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	sessionRequest = checkout.CreateCheckoutSessionRequest{
		AllowedPaymentMethods:  []string{"ideal", "scheme"},
		Amount:                 checkout.Amount{Value: 12300, Currency: "EUR"},
		Channel:                "Web",
		CountryCode:            "nl",
		MerchantAccount:        "MyMerchantAccount",
		MerchantOrderReference: "123",
		Reference:              "123",
		ReturnUrl:              "http:///checkout/123",
		ShopperLocale:          "nl-nl",
		TrustedShopper:         true,
	}
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
	paymentMethodsReq = checkout.PaymentMethodsRequest{
		Amount:          &checkout.Amount{Value: 12300, Currency: "EUR"},
		Channel:         "Web",
		CountryCode:     "nl",
		MerchantAccount: "MyMerchantAccount",
		ShopperLocale:   "nl-nl",
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
		payer.EXPECT().Sessions(gomock.Any(), &sessionRequest).Return(&sessionResp, nil)
		payer.EXPECT().PaymentMethods(gomock.Any(), &paymentMethodsReq).Return(&paymentMethodsResp, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)

		// when
		request, _ := http.NewRequest(http.MethodPost, "/checkout/123", strings.NewReader(`amount=12300&currency=EUR&returnUrl=http://a.b/c&countryCode=nl&shopper.locale=nl-nl`))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td>123</td>")
		assert.Contains(t, got, `id: "456"`)
		assert.Contains(t, got, `sessionData: "lalalallalalaallalalalalalal"`)

		checkout, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		assert.Equal(t, "123", checkout.BasketUID)
		assert.Equal(t, "456", checkout.ID)
		assert.Equal(t, "lalalallalalaallalalalalalal", checkout.SessionData)
		assert.Equal(t, "http://a.b/c", checkout.OriginalReturnURL)
	})

	t.Run("Handle checkout status redirect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, _, nower := setup(ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		storer.Put(ctx, "123", checkoutmodel.CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		// when
		request, _ := http.NewRequest(http.MethodGet, "/checkout/123/status/success", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "http://localhost:8080/basket/123/checkout?status=success", redirectURL)

		checkout, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		assert.Equal(t, "123", checkout.BasketUID)
		assert.Equal(t, "success", checkout.Status)
	})

	t.Run("Handle checkout status webhook", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, queuer, nower := setup(ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime.Add(time.Hour))
		queuer.EXPECT().Enqueue(gomock.Any(), gomock.Any()).Return(nil)

		storer.Put(ctx, "123", checkoutmodel.CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		// when
		request, _ := http.NewRequest(http.MethodPost, "/checkout/webhook/event", strings.NewReader(`{
   "live":"false",
   "notificationItems":[
      {
         "NotificationRequestItem":{
            "eventCode":"AUTHORISATION",
            "success":"true",
            "eventDate":"2019-06-28T18:03:50+01:00",
            "merchantAccountCode":"MyMerchantAccount",
            "pspReference": "7914073381342284",
            "merchantReference": "123",
            "amount": {
                "value":12300,
                "currency":"EUR"
            }
         }
      }
   ]
}`))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Equal(t, `{
	"status": "[accepted]"
}
`, got)

		checkout, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		assert.Equal(t, "123", checkout.BasketUID)
		assert.Equal(t, "AUTHORISATION", checkout.WebhookStatus)
		assert.Equal(t, "true", checkout.WebhookSuccess)
	})
}

func setup(ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[checkoutmodel.CheckoutContext], *MockPayer, *myqueue.MockTaskQueuer, *mytime.MockNower) {
	c := context.TODO()
	storer, _, _ := mystore.New[checkoutmodel.CheckoutContext](c)
	nower := mytime.NewMockNower(ctrl)
	queuer := myqueue.NewMockTaskQueuer(ctrl)
	payer := NewMockPayer(ctrl)

	sut, _ := NewService(Config{
		Environment:     "Test",
		MerchantAccount: "MyMerchantAccount",
		ClientKey:       "my_client_key",
		ApiKey:          "my_api_key",
	}, payer, storer, queuer, nower, mylog.New("checkout"))
	router := mux.NewRouter()
	sut.RegisterEndpoints(c, router)

	return c, router, storer, payer, queuer, nower
}