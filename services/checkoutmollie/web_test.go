package checkoutmollie

import (
	"context"
	"time"

	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VictorAvelar/mollie-api-go/v3/mollie"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
)

var (
	paymentRequest = mollie.Payment{
		ConsumerName: "Marc Grol",
		WebhookURL:   "http://localhost:8888/mollie/checkout/webhook/event/",
		Description:  "Goods ordered in basket",
		RedirectURL:  "http://localhost:8888/mollie/checkout//status/success",
		CancelURL:    "http://localhost:8888/mollie/checkout//status/cancelled",
		Metadata: map[string]string{
			"basketUID": "xxx",
		},
		Amount: &mollie.Amount{
			Currency: "EUR",
			Value:    "123.00",
		},
		BillingAddress: &mollie.Address{
			StreetAndNumber: "",
			City:            "",
			Region:          "",
			PostalCode:      "",
			Country:         "",
		},
		ShippingAddress: &mollie.PaymentDetailsAddress{
			StreetAndNumber: "",
			City:            "",
			Region:          "",
			PostalCode:      "",
			Country:         "",
		},
		Locale: "nl_NL",
	}

	paymentResponse = mollie.Payment{
		ConsumerName: "Marc Grol",
		WebhookURL:   "http://localhost:8888/mollie/checkout/webhook/event/",
		Description:  "Goods ordered in basket",
		RedirectURL:  "http://localhost:8888/mollie/checkout//status/success",
		CancelURL:    "http://localhost:8888/mollie/checkout//status/cancelled",
		Metadata: map[string]string{
			"basketUID": "xxx",
		},
		Amount: &mollie.Amount{
			Currency: "EUR",
			Value:    "123.00",
		},
		BillingAddress: &mollie.Address{
			StreetAndNumber: "",
			City:            "",
			Region:          "",
			PostalCode:      "",
			Country:         "",
		},
		ShippingAddress: &mollie.PaymentDetailsAddress{
			StreetAndNumber: "",
			City:            "",
			Region:          "",
			PostalCode:      "",
			Country:         "",
		},
		Locale: "nl_NL",
		Links: mollie.PaymentLinks{
			Checkout: &mollie.URL{
				Href: "https://checkout.test.mollie.com/checkout",
			},
		},
	}
)

func TestCheckoutService(t *testing.T) {

	t.Run("Create checkout with api-key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, vault, payer, nower, _, publisher := setup(t, ctrl)

		// given
		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken+"_"+"mollie").Return(myvault.Token{}, false, nil)
		payer.EXPECT().UseAPIKey("my_api_key")
		payer.EXPECT().CreatePayment(gomock.Any(), gomock.Any()).Return(paymentResponse, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName:  "mollie",
			CheckoutUID:   "123",
			AmountInCents: 12300,
			Currency:      "EUR",
			ShopperUID:    "xyz",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/mollie/checkout/123", strings.NewReader(`totalAmount.value=12300&totalAmount.currency=EUR&company.countryCode=nl&shopper.locale=nl-nl&shopper.uid=xyz&shopper.firstName=Marc&shopper.lastName=Grol&returnUrl=http://a.b/c`))
		assert.NoError(t, err)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "https://checkout.test.mollie.com/checkout", redirectURL)

	})

	t.Run("Create checkout with access-token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, _, vault, payer, nower, _, publisher := setup(t, ctrl)

		// given
		vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken+"_"+"mollie").Return(myvault.Token{
			ProviderName: "mollie",
			AccessToken:  "my_access_token",
			SessionUID:   "my_oauth_session_uid",
		}, true, nil)
		payer.EXPECT().UseToken("my_access_token")
		payer.EXPECT().CreatePayment(gomock.Any(), gomock.Any()).Return(paymentResponse, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutStarted{
			ProviderName:  "mollie",
			CheckoutUID:   "123",
			AmountInCents: 12300,
			Currency:      "EUR",
			ShopperUID:    "xyz",
		}).Return(nil)

		// when
		request, err := http.NewRequest(http.MethodPost, "/mollie/checkout/123", strings.NewReader(`totalAmount.value=12300&totalAmount.currency=EUR&company.countryCode=nl&shopper.locale=nl-nl&shopper.uid=xyz&shopper.firstName=Marc&shopper.lastName=Grol&returnUrl=http://a.b/c`))
		assert.NoError(t, err)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 303, response.Code)
		redirectURL := response.Header().Get("Location")
		assert.Equal(t, "https://checkout.test.mollie.com/checkout", redirectURL)
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
		request, err := http.NewRequest(http.MethodGet, "/mollie/checkout/123/status/success", nil)
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
		ctx, router, storer, _, payer, nower, _, publisher := setup(t, ctrl)

		// given
		//vault.EXPECT().Get(gomock.Any(), myvault.CurrentToken+"_"+"mollie").Return(myvault.Token{}, false, nil)
		payer.EXPECT().UseAPIKey("my_api_key")
		payer.EXPECT().GetPaymentOnID(gomock.Any(), "xyz").Return(mollie.Payment{
			ID:     "xyz",
			Status: "paid",
			Method: "ideal",
		}, nil)
		nower.EXPECT().Now().Return(mytime.ExampleTime.Add(time.Hour))
		publisher.EXPECT().Publish(gomock.Any(), checkoutevents.TopicName, checkoutevents.CheckoutCompleted{
			ProviderName:          "mollie",
			CheckoutUID:           "123",
			PaymentMethod:         "ideal",
			CheckoutStatus:        checkoutevents.CheckoutStatusSuccess,
			CheckoutStatusDetails: "paid",
		}).Return(nil)

		_ = storer.Put(ctx, "123", checkoutapi.CheckoutContext{
			BasketUID:         "123",
			CreatedAt:         mytime.ExampleTime.Add(-1 * (time.Hour)),
			LastModified:      nil,
			OriginalReturnURL: "http://localhost:8080/basket/123/checkout",
			ID:                "456",
			SessionData:       "lalala",
		})

		request, err := http.NewRequest(http.MethodPost, "/mollie/checkout/webhook/event/123", strings.NewReader("id=xyz"))
		request.Header["Content-Type"] = []string{"application/x-www-form-urlencoded"}
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)

		checkout, exists, _ := storer.Get(ctx, "123")
		assert.True(t, exists)
		t.Logf("*** resp: %+v", checkout)
		assert.Equal(t, "123", checkout.BasketUID)
		assert.Equal(t, checkoutevents.CheckoutStatusSuccess, checkout.CheckoutStatus)
		assert.Equal(t, "paid", checkout.CheckoutStatusDetails)
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
