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

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/shop/shopevents"
)

var (
	basket1 = Basket{UID: "123", CreatedAt: mytime.ExampleTime, TotalPrice: 100, Currency: "EUR", InitialPaymentStatus: "success"}
)

func TestBasketService(t *testing.T) {

	t.Run("List baskets", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, storer, _, _, _ := setup(t, ctrl)

		// given
		basket2 := Basket{UID: "456", CreatedAt: mytime.ExampleTime.Add(time.Minute), TotalPrice: 200, Currency: "EUR", InitialPaymentStatus: "success", CheckoutStatus: string(checkoutevents.CheckoutStatusSuccess), CheckoutStatusDetails: "AUTHORIZED=true"}
		storer.EXPECT().List(gomock.Any()).Return([]Basket{basket1, basket2}, nil)

		// when
		request, err := http.NewRequest(http.MethodGet, "/basket", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td><a href=\"/basket/123\"></a></td>")
		assert.Contains(t, got, "<td><a href=\"/basket/456\"></a></td>")
	})

	t.Run("Get basket", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// given
		_, router, storer, _, _, _ := setup(t, ctrl)

		// given
		storer.EXPECT().Get(gomock.Any(), "123").Return(basket1, true, nil)

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
		_, router, storer, _, _, _ := setup(t, ctrl)
		storer.EXPECT().Get(gomock.Any(), "123").Return(Basket{}, false, nil)

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
		_, router, storer, nower, uuider, publisher := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		uuider.EXPECT().Create().Return("123")
		storer.EXPECT().RunInTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, f func(ctx context.Context) error) error {
				return f(ctx)
			})
		storer.EXPECT().Put(gomock.Any(), "123", gomock.Any()).DoAndReturn(
			func(ctx context.Context, uid string, basket Basket) error {
				assert.Equal(t, "123", basket.UID)
				assert.Equal(t, mytime.ExampleTime, basket.CreatedAt)
				assert.Equal(t, "http://localhost:8888/basket/123/checkout/completed", basket.ReturnURL)
				return nil
			})
		publisher.EXPECT().Publish(gomock.Any(), shopevents.TopicName, shopevents.BasketCreated{BasketUID: "123"}).Return(nil)

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
	})

	t.Run("Handle status redirect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, storer, nower, _, _ := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		storer.EXPECT().RunInTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, f func(ctx context.Context) error) error {
				return f(ctx)
			})
		storer.EXPECT().Get(gomock.Any(), "123").Return(basket1, true, nil)
		storer.EXPECT().Put(gomock.Any(), "123", gomock.Any()).DoAndReturn(
			func(ctx context.Context, uid string, basket Basket) error {

				assert.Equal(t, "123", basket.UID)
				assert.Equal(t, "completed", basket.InitialPaymentStatus)
				assert.NotNil(t, basket.LastModified)

				return nil
			})

		// when
		request, err := http.NewRequest(http.MethodGet, "/basket/123/checkout/completed", nil)
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		// then
		assert.Equal(t, 200, response.Code)
		got := response.Body.String()
		assert.Contains(t, got, "<td>123</td>")
	})

	t.Run("Handle async update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// setup
		_, router, storer, nower, _, publisher := setup(t, ctrl)

		// given
		nower.EXPECT().Now().Return(mytime.ExampleTime)
		storer.EXPECT().RunInTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, f func(ctx context.Context) error) error {
				return f(ctx)
			})
		storer.EXPECT().Get(gomock.Any(), "123").Return(basket1, true, nil)
		storer.EXPECT().Put(gomock.Any(), "123", gomock.Any()).DoAndReturn(
			func(ctx context.Context, uid string, basket Basket) error {

				assert.Equal(t, "123", basket.UID)
				assert.Equal(t, "ideal", basket.PaymentMethod)
				assert.True(t, basket.Done)
				assert.Equal(t, "success", basket.CheckoutStatus)
				assert.Equal(t, "AUTHORIZED=true", basket.CheckoutStatusDetails)

				return nil
			})
		publisher.EXPECT().Publish(gomock.Any(), shopevents.TopicName,
			shopevents.BasketPaymentCompleted{BasketUID: basket1.UID})

		// when
		request, err := http.NewRequest(http.MethodPost, "/api/basket/event", strings.NewReader(createPubsubMessage(
			checkoutevents.CheckoutCompleted{
				CheckoutUID:           "123",
				PaymentMethod:         "ideal",
				CheckoutStatus:        checkoutevents.CheckoutStatusSuccess,
				CheckoutStatusDetails: "AUTHORIZED=true",
			})))
		assert.NoError(t, err)
		request.Host = "localhost:8888"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

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

func setup(t *testing.T, ctrl *gomock.Controller) (context.Context, *mux.Router, *mystore.MockStore[Basket], *mytime.MockNower, *myuuid.MockUUIDer, *mypublisher.MockPublisher) {
	c := context.TODO()
	storer := mystore.NewMockStore[Basket](ctrl)
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
