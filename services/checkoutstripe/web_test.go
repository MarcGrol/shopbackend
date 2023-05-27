package checkoutstripe

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
	"github.com/stripe/stripe-go/v74"

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
)

var (
	/*
			Fixes needed to replace gomock.Any with sessionRequest:
		v	Got: {{<nil> [] <nil> map[] <nil> map[basketUID:123] <nil>} <nil> <nil> <nil> <nil> 0xc0003692d0 0xc000369310 <nil> 0xc000369330 <nil> <nil> 0xc000369340 <nil> [] <nil> [] <nil> <nil> [] 0xc000369350 0xc000369320 0xc000127260 <nil> <nil> [0xc000157600 0xc000157610] <nil> <nil> <nil> [] [] <nil> <nil> 0xc000369290 <nil>} (stripe.CheckoutSessionParams)
			Want: is equal to {{<nil> [] <nil> map[] <nil> map[basketUID:123] <nil>} <nil> <nil> <nil> <nil> 0xc000368a10 0xc000368a20 <nil> 0xc000368a40 <nil> <nil> 0xc000368a50 <nil> [] <nil> [] <nil> <nil> [] 0xc000368a60 0xc000368a30 0x1eddca0 <nil> <nil> [0xc000157200 0xc000157210] <nil> <nil> <nil> [] [] <nil> <nil> 0xc000368a00 <nil>} (stripe.CheckoutSessionParams)

			sessionRequest = stripe.CheckoutSessionParams{
				Params: stripe.Params{
					Metadata: map[string]string{
						"basketUID": "123",
					},
				},
				PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
					Metadata: map[string]string{
						"basketUID": "123", // This is to correlare the webhook with the basket
					},
					Shipping: &stripe.ShippingDetailsParams{
						Name:           stripe.String("Marc Grol"),
						Phone:          stripe.String("31612345678"),
						TrackingNumber: stripe.String("123"),
						Address: &stripe.AddressParams{
							City:       stripe.String("Utrecht"),
							Country:    stripe.String("NL"),
							Line1:      stripe.String("My street 79"),
							PostalCode: stripe.String("1234AB"),
						},
					},
				},
				SuccessURL:         stripe.String("/stripe/checkout/123/status/success"),
				CancelURL:          stripe.String("/stripe/checkout/123/status/cancel"),
				ClientReferenceID:  stripe.String("123"),
				LineItems:          []*stripe.CheckoutSessionLineItemParams{},
				Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
				Currency:           stripe.String("EUR"),
				CustomerEmail:      stripe.String("my@email.com"),
				Locale:             stripe.String("nl"),
				PaymentMethodTypes: stripe.StringSlice([]string{"ideal", "card"}),
			}
	*/
	sessionResp = stripe.CheckoutSession{
		ID:          "456",
		AmountTotal: int64(12300),
		Currency:    "EUR",
		URL:         "http://my_url.com",
	}
)

func TestCheckoutService(t *testing.T) {

	t.Run("Create checkout with api-key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, vault, payer, nower, _, publisher := setup(t, ctrl)

		// given
		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken+"_"+"stripe").Return(myvault.Token{}, false, nil)
		payer.EXPECT().UseAPIKey("my_api_key")
		payer.EXPECT().CreateCheckoutSession(gomock.Any(), gomock.Any()).Return(sessionResp, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName:  "stripe",
			CheckoutUID:   "123",
			AmountInCents: 12300,
			Currency:      "EUR",
			ShopperUID:    "123",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/stripe/checkout/123", strings.NewReader(`totalAmount.value=12300&totalAmount.currency=EUR&company.countryCode=nl&shopper.locale=nl-nl&shopper.firstName=Marc&shopper.lastName=Grol&returnUrl=http://a.b/c`))
		assert.NoError(t, err)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "http://my_url.com", redirectURL)

	})

	t.Run("Create checkout with access-token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, vault, payer, nower, _, publisher := setup(t, ctrl)

		// given
		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken+"_"+"stripe").Return(myvault.Token{
			ProviderName: "stripe",
			AccessToken:  "my_access_token",
			SessionUID:   "my_oauth_session_uid",
		}, true, nil)
		payer.EXPECT().UseToken("my_access_token")
		payer.EXPECT().CreateCheckoutSession(gomock.Any(), gomock.Any()).Return(sessionResp, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName:  "stripe",
			CheckoutUID:   "123",
			AmountInCents: 12300,
			Currency:      "EUR",
			ShopperUID:    "123",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/stripe/checkout/123", strings.NewReader(`totalAmount.value=12300&totalAmount.currency=EUR&company.countryCode=nl&shopper.locale=nl-nl&shopper.firstName=Marc&shopper.lastName=Grol&returnUrl=http://a.b/c`))
		assert.NoError(t, err)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "http://my_url.com", redirectURL)

	})

	t.Run("Handle checkout status redirect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, _, nower, _, _ := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		_ = storer.Put(ctx, "123", checkoutapi.CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		// when
		request, err := http.NewRequest(http.MethodGet, "/stripe/checkout/123/status/success", nil)
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
			ProviderName:  "stripe",
			CheckoutUID:   "123",
			PaymentMethod: "ideal",
			Status:        "payment_intent.succeeded",
			Success:       true,
		}).Return(nil)

		_ = storer.Put(ctx, "123", checkoutapi.CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		// when
		request, err := http.NewRequest(http.MethodPost, "/stripe/checkout/webhook/event", strings.NewReader(`{
			"id": "evt_2Zj5zzFU3a9abcZ1aYYYaaZ1",
			"object": "event",
			"api_version": "2022-11-15",
			"created": 1633887337,
			"type": "payment_intent.succeeded",
			"data": {
				"object": {
					"metadata": {
						"basketUID": "123"
					  },
					  "payment_method_types": ["ideal" ]
				}
			}
}`))
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)

		checkout, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		assert.Equal(t, "123", checkout.BasketUID)
		assert.Equal(t, "payment_intent.succeeded", checkout.WebhookEventName)
		assert.True(t, checkout.WebhookEventSuccess)
	})
}

func setup(t *testing.T, ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[checkoutapi.CheckoutContext], *myvault.MockVaultReader, *MockPayer, *mytime.MockNower, *mypubsub.MockPubSub, *mypublisher.MockPublisher) {
	c := context.TODO()
	storer, _, _ := mystore.New[checkoutapi.CheckoutContext](c)
	vault := myvault.NewMockVaultReader(ctrl)
	nower := mytime.NewMockNower(ctrl)
	payer := NewMockPayer(ctrl)
	subscriber := mypubsub.NewMockPubSub(ctrl)
	publisher := mypublisher.NewMockPublisher(ctrl)

	sut, err := NewWebService("my_api_key", payer, nower, storer, vault, publisher)
	assert.NoError(t, err)
	router := mux.NewRouter()

	// These are called by the following call to RegisterEndpoints
	//publisher.EXPECT().CreateTopic(gomock.Any(), checkoutevents.TopicName).Return(nil)
	// subscriber.EXPECT().CreateTopic(c, oauthevents.TopicName).Return(nil)
	// subscriber.EXPECT().Subscribe(c, oauthevents.TopicName, "http://localhost:8080/api/adyen/checkout/event").Return(nil)

	err = sut.RegisterEndpoints(c, router)
	assert.NoError(t, err)

	return c, router, storer, vault, payer, nower, subscriber, publisher
}
