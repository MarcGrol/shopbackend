package shop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MarcGrol/shopbackend/lib/myevents"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/shop/shopevents"
)

var (
	basket1 = Basket{UID: "123", CreatedAt: time.Now(), TotalPrice: 100, Currency: "EUR", InitialPaymentStatus: "success", FinalPaymentEvent: ""}
	basket2 = Basket{UID: "456", CreatedAt: time.Now().Add(time.Minute), TotalPrice: 200, Currency: "EUR", InitialPaymentStatus: "success", FinalPaymentEvent: "AUTHORISATION", FinalPaymentStatus: true}
)

func TestBasketService(t *testing.T) {

	t.Run("List baskets", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		ctx, router, storer, _, _, _ := setup(t, ctrl)

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
		ctx, router, storer, _, _, _ := setup(t, ctrl)

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
		_, router, _, _, _, _ := setup(t, ctrl)

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
		ctx, router, storer, nower, uuider, publisher := setup(t, ctrl)

		// given
		storer.Put(ctx, basket1.UID, basket1)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("123")
		publisher.EXPECT().Publish(gomock.Any(), shopevents.TopicName, shopevents.BasketCreated{BasketUID: "123"})

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
		ctx, router, storer, nower, _, _ := setup(t, ctrl)

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
		ctx, router, storer, nower, _, publisher := setup(t, ctrl)

		// given
		storer.Put(ctx, basket1.UID, basket1)
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		publisher.EXPECT().Publish(gomock.Any(), shopevents.TopicName,
			shopevents.BasketPaymentCompleted{BasketUID: basket1.UID})

		// when
		request, err := http.NewRequest(http.MethodPost, "/api/basket/event", strings.NewReader(createPubsubMessage(
			checkoutevents.CheckoutCompleted{
				CheckoutUID:   "123",
				PaymentMethod: "ideal",
				Status:        "AUTHORIZED",
				Success:       true,
			})))
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
	})
}

func createPubsubMessage(event checkoutevents.CheckoutCompleted) string {
	eventBytes, _ := json.Marshal(event)
	envelope := myevents.EventEnvelope{
		UID:           "123",
		CreatedAt:     mytime.ExampleTime,
		Topic:         "checkout",
		AggregateUID:  "111",
		EventTypeName: "checkout.completed",
		EventPayload:  string(eventBytes),
	}
	envelopeBytes, _ := json.Marshal(envelope)

	req := myevents.PushRequest{
		Message: myevents.PushMessage{
			Data: envelopeBytes,
		},
		Subscription: "checkout",
	}

	reqBytes, _ := json.Marshal(req)

	return string(reqBytes)
}

func setup(t *testing.T, ctrl *gomock.Controller) (context.Context, *mux.Router, mystore.Store[Basket], *mytime.MockNower, *myuuid.MockUUIDer, *mypublisher.MockPublisher) {
	c := context.TODO()
	storer, _, _ := mystore.New[Basket](c)
	nower := mytime.NewMockNower(ctrl)
	uuider := myuuid.NewMockUUIDer(ctrl)
	subscriber := mypubsub.NewMockPubSub(ctrl)
	publisher := mypublisher.NewMockPublisher(ctrl)

	sut := NewService(storer, nower, uuider, subscriber, publisher)
	router := mux.NewRouter()

	// These are called by the following call to RegisterEndpoints()
	publisher.EXPECT().CreateTopic(c, shopevents.TopicName).Return(nil)
	subscriber.EXPECT().CreateTopic(c, checkoutevents.TopicName).Return(nil)
	subscriber.EXPECT().Subscribe(c, checkoutevents.TopicName, "http://localhost:8080/api/basket/event").Return(nil)

	err := sut.RegisterEndpoints(c, router)
	assert.NoError(t, err)

	return c, router, storer, nower, uuider, publisher
}
