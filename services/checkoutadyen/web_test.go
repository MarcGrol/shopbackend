package checkoutadyen

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

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
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
		ReturnUrl:              "http://localhost:8888/checkout/123",
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
		ctx, router, storer, vault, payer, nower, _, publisher := setup(t, ctrl)

		// given
		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken)
		payer.EXPECT().UseApiKey(gomock.Any())
		payer.EXPECT().Sessions(gomock.Any(), sessionRequest).Return(sessionResp, nil)
		payer.EXPECT().PaymentMethods(gomock.Any(), paymentMethodsReq).Return(paymentMethodsResp, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName:  "adyen",
			CheckoutUID:   "123",
			AmountInCents: 12300,
			Currency:      "EUR",
			MerchantUID:   "MyMerchantAccount",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/checkout/123", strings.NewReader(`amount=12300&currency=EUR&returnUrl=http://a.b/c&countryCode=nl&shopper.locale=nl-nl`))
		assert.NoError(t, err)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Host = "localhost:8888"
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

	t.Run("Resume checkout", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, _, _, _, _ := setup(t, ctrl)

		// given
		storer.Put(ctx, "123", CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		// when
		request, err := http.NewRequest(http.MethodGet, "/checkout/123", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td>123</td>")
		assert.Contains(t, got, `id: "456"`)
		assert.Contains(t, got, `sessionData: "lalala"`)

		checkout, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		assert.Equal(t, "123", checkout.BasketUID)
		assert.Equal(t, "456", checkout.ID)
		assert.Equal(t, "lalala", checkout.SessionData)
		assert.Equal(t, "http://localhost:8080/basket/123/checkout", checkout.OriginalReturnURL)
	})

	t.Run("Handle checkout status redirect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, _, nower, _, _ := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		storer.Put(ctx, "123", CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		// when
		request, err := http.NewRequest(http.MethodGet, "/checkout/123/status/success", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
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
		ctx, router, storer, _, _, nower, _, publisher := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime.Add(time.Hour))
		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
			ProviderName:  "adyen",
			CheckoutUID:   "123",
			PaymentMethod: "ideal",
			Status:        "AUTHORISATION",
			Success:       true,
		}).Return(nil)

		storer.Put(ctx, "123", CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		// when
		request, err := http.NewRequest(http.MethodPost, "/checkout/webhook/event", strings.NewReader(`{
   "live":"false",
   "notificationItems":[
      {
         "NotificationRequestItem":{
            "eventCode":"AUTHORISATION",
			"paymentMethod":"ideal",
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
		assert.NoError(t, err)
		request.Host = "localhost:8888"
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

func setup(t *testing.T, ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[CheckoutContext], *myvault.MockVaultReader, *MockPayer, *mytime.MockNower, *mypubsub.MockPubSub, *mypublisher.MockPublisher) {
	c := context.TODO()
	storer, _, _ := mystore.New[CheckoutContext](c)
	vault := myvault.NewMockVaultReader(ctrl)
	nower := mytime.NewMockNower(ctrl)
	payer := NewMockPayer(ctrl)
	subscriber := mypubsub.NewMockPubSub(ctrl)
	publisher := mypublisher.NewMockPublisher(ctrl)

	sut, err := NewWebService(Config{
		Environment:     "Test",
		MerchantAccount: "MyMerchantAccount",
		ClientKey:       "my_client_key",
		ApiKey:          "my_api_key",
	}, payer, storer, vault, nower, subscriber, publisher)
	assert.NoError(t, err)
	router := mux.NewRouter()

	// These are called by the following call to RegisterEndpoints
	publisher.EXPECT().CreateTopic(c, checkoutevents.TopicName).Return(nil)
	subscriber.EXPECT().CreateTopic(c, oauthevents.TopicName).Return(nil)
	subscriber.EXPECT().Subscribe(c, oauthevents.TopicName, "http://localhost:8080/api/checkout/event").Return(nil)

	err = sut.RegisterEndpoints(c, router)
	assert.NoError(t, err)

	return c, router, storer, vault, payer, nower, subscriber, publisher
}
