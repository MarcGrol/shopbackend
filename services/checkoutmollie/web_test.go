package checkoutmollie

// import (
// 	"context"

// 	"net/http"
// 	"net/http/httptest"
// 	"strings"
// 	"testing"
// 	"time"

// 	"github.com/VictorAvelar/mollie-api-go/v3/mollie"
// 	"github.com/golang/mock/gomock"
// 	"github.com/gorilla/mux"
// 	"github.com/stretchr/testify/assert"

// 	"github.com/MarcGrol/shopbackend/lib/mypublisher"
// 	"github.com/MarcGrol/shopbackend/lib/mypubsub"
// 	"github.com/MarcGrol/shopbackend/lib/mystore"
// 	"github.com/MarcGrol/shopbackend/lib/mytime"
// 	"github.com/MarcGrol/shopbackend/lib/myvault"
// 	"github.com/MarcGrol/shopbackend/services/checkoutapi"
// 	"github.com/MarcGrol/shopbackend/services/checkoutevents"
// )

// var (
// 	paymentRequest = mollie.Payment{}

// 	paymentResponse = mollie.Payment{}
// )

// func TestCheckoutService(t *testing.T) {

// 	t.Run("Create checkout with api-key", func(t *testing.T) {
// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		// setup
// 		_, router, _, vault, payer, nower, _, publisher := setup(t, ctrl)

// 		// given
// 		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken+"_"+"mollie").Return(myvault.Token{}, false, nil)
// 		payer.EXPECT().UseAPIKey("my_api_key")
// 		payer.EXPECT().CreatePayment(gomock.Any(), paymentRequest).Return(paymentResponse, nil)
// 		nower.EXPECT().Now().Return(mytime.ExampleTime)
// 		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutStarted{
// 			ProviderName:  "mollie",
// 			CheckoutUID:   "123",
// 			AmountInCents: 12300,
// 			Currency:      "EUR",
// 			ShopperUID:    "123",
// 		}).Return(nil)

// 		// when
// 		request, err := http.NewRequest(http.MethodPost, "/mollie/checkout/123", strings.NewReader(`totalAmount.value=12300&totalAmount.currency=EUR&company.countryCode=nl&shopper.locale=nl-nl&shopper.firstName=Marc&shopper.lastName=Grol&returnUrl=http://a.b/c`))
// 		assert.NoError(t, err)
// 		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
// 		request.Host = "localhost:8888"
// 		response := httptest.NewRecorder()
// 		router.ServeHTTP(response, request)

// 		// then
// 		assert.Equal(t, 303, response.Code)
// 		redirectURL := response.Header().Get("Location")
// 		assert.Equal(t, "http://my_url.com", redirectURL)

// 	})

// 	t.Run("Create checkout with access-token", func(t *testing.T) {
// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		// setup
// 		_, router, _, vault, payer, nower, _, publisher := setup(t, ctrl)

// 		// given
// 		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken+"_"+"mollie").Return(myvault.Token{
// 			ProviderName: "mollie",
// 			AccessToken:  "my_access_token",
// 			SessionUID:   "my_oauth_session_uid",
// 		}, true, nil)
// 		payer.EXPECT().UseToken("my_access_token")
// 		payer.EXPECT().CreatePayment(gomock.Any(), paymentRequest).Return(paymentResponse, nil)
// 		nower.EXPECT().Now().Return(mytime.ExampleTime)
// 		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutStarted{
// 			ProviderName:  "mollie",
// 			CheckoutUID:   "123",
// 			AmountInCents: 12300,
// 			Currency:      "EUR",
// 			ShopperUID:    "123",
// 		}).Return(nil)

// 		// when
// 		request, err := http.NewRequest(http.MethodPost, "/mollie/checkout/123", strings.NewReader(`totalAmount.value=12300&totalAmount.currency=EUR&company.countryCode=nl&shopper.locale=nl-nl&shopper.firstName=Marc&shopper.lastName=Grol&returnUrl=http://a.b/c`))
// 		assert.NoError(t, err)
// 		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
// 		request.Host = "localhost:8888"
// 		response := httptest.NewRecorder()
// 		router.ServeHTTP(response, request)

// 		// then
// 		assert.Equal(t, 303, response.Code)
// 		redirectURL := response.Header().Get("Location")
// 		assert.Equal(t, "http://my_url.com", redirectURL)

// 	})

// 	t.Run("Handle checkout status redirect", func(t *testing.T) {
// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		// setup
// 		ctx, router, storer, _, _, nower, _, _ := setup(t, ctrl)

// 		// given
// 		nower.EXPECT().Now().Return(mytime.ExampleTime)
// 		_ = storer.Put(ctx, "123", checkoutapi.CheckoutContext{
// 			BasketUID:         "123",
// 			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
// 			LastModified:      nil,
// 			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
// 			ID:                "456",
// 			SessionData:       "lalala",
// 		})

// 		// when
// 		request, err := http.NewRequest(http.MethodGet, "/mollie/checkout/123/status/success", nil)
// 		assert.NoError(t, err)
// 		request.Host = "localhost:8888"
// 		response := httptest.NewRecorder()
// 		router.ServeHTTP(response, request)

// 		// then
// 		assert.Equal(t, 303, response.Code)
// 		redirectURL := response.Header().Get("Location")
// 		assert.Equal(t, "http://localhost:8080/basket/123/checkout?status=success", redirectURL)

// 		checkout, exists, _ := storer.Get(ctx, "123")
// 		assert.True(t, exists)
// 		assert.Equal(t, "123", checkout.BasketUID)
// 		assert.Equal(t, "success", checkout.Status)
// 	})

// 	t.Run("Handle checkout status webhook", func(t *testing.T) {
// 		ctrl := gomock.NewController(t)
// 		defer ctrl.Finish()

// 		// setup
// 		ctx, router, storer, _, _, nower, _, publisher := setup(t, ctrl)

// 		// given
// 		nower.EXPECT().Now().Return(mytime.ExampleTime.Add(time.Hour))
// 		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
// 			ProviderName:          "mollie",
// 			CheckoutUID:           "123",
// 			PaymentMethod:         "ideal",
// 			CheckoutStatus:        checkoutevents.CheckoutStatusSuccess,
// 			CheckoutStatusDetails: "payment_intent.succeeded",
// 		}).Return(nil)

// 		_ = storer.Put(ctx, "123", checkoutapi.CheckoutContext{
// 			BasketUID:         "123",
// 			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
// 			LastModified:      nil,
// 			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
// 			ID:                "456",
// 			SessionData:       "lalala",
// 		})

// 		// when
// 		request, err := http.NewRequest(http.MethodPost, "/mollie/checkout/webhook/event", strings.NewReader(`{
// 			"id": "evt_2Zj5zzFU3a9abcZ1aYYYaaZ1",
// 			"object": "event",
// 			"api_version": "2022-11-15",
// 			"created": 1633887337,
// 			"type": "payment_intent.succeeded",
// 			"data": {
// 				"object": {
// 					"metadata": {
// 						"basketUID": "123"
// 					  },
// 					  "payment_method_types": ["ideal" ]
// 				}
// 			}
// }`))
// 		assert.NoError(t, err)
// 		request.Host = "localhost:8888"
// 		response := httptest.NewRecorder()
// 		router.ServeHTTP(response, request)

// 		// then
// 		assert.Equal(t, 200, response.Code)

// 		checkout, exists, _ := storer.Get(ctx, "123")
// 		assert.True(t, exists)
// 		assert.Equal(t, "123", checkout.BasketUID)
// 		assert.Equal(t, checkoutevents.CheckoutStatusSuccess, checkout.CheckoutStatus)
// 		assert.Equal(t, "payment_intent.succeeded", checkout.CheckoutStatusDetails)
// 	})
// }

// func setup(t *testing.T, ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[checkoutapi.CheckoutContext], *myvault.MockVaultReader, *MockPayer, *mytime.MockNower, *mypubsub.MockPubSub, *mypublisher.MockPublisher) {
// 	c := context.TODO()
// 	storer, _, _ := mystore.New[checkoutapi.CheckoutContext](c)
// 	vault := myvault.NewMockVaultReader(ctrl)
// 	nower := mytime.NewMockNower(ctrl)
// 	payer := NewMockPayer(ctrl)
// 	subscriber := mypubsub.NewMockPubSub(ctrl)
// 	publisher := mypublisher.NewMockPublisher(ctrl)

// 	sut, err := NewWebService("my_api_key", payer, nower, storer, vault, publisher)
// 	assert.NoError(t, err)
// 	router := mux.NewRouter()

// 	// These are called by the following call to RegisterEndpoints
// 	//publisher.EXPECT().CreateTopic(gomock.Any(), checkoutevents.TopicName).Return(nil)
// 	// subscriber.EXPECT().CreateTopic(c, oauthevents.TopicName).Return(nil)
// 	// subscriber.EXPECT().Subscribe(c, oauthevents.TopicName, "http://localhost:8080/api/adyen/checkout/event").Return(nil)

// 	err = sut.RegisterEndpoints(c, router)
// 	assert.NoError(t, err)

// 	return c, router, storer, vault, payer, nower, subscriber, publisher
// }
