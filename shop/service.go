package shop

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/shop/shopmodel"
)

type service struct {
	basketStore mystore.Store[shopmodel.Basket]
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(store mystore.Store[shopmodel.Basket], nower mytime.Nower, uuider myuuid.UUIDer, logger mylog.Logger) *service {
	return &service{
		basketStore: store,
		nower:       nower,
		uuider:      uuider,
		logger:      logger,
	}
}

func (s service) subscribe(c context.Context) error {
	// projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	// client, err := pubsub.NewClient(c, projectId)
	// if err != nil {
	// 	return fmt.Errorf("error creating client: %s", err)
	// }
	// defer client.Close()

	// _, err = client.CreateSubscription(c, checkout.TopicName, pubsub.SubscriptionConfig{
	// 	N
	// })
	// if err != nil {
	// 	return fmt.Errorf("error creating subscription %s: %s", checkout.TopicName, err)
	// }

	return nil
}

func (s service) listBaskets(c context.Context) ([]shopmodel.Basket, error) {
	s.logger.Log(c, "", mylog.SeverityInfo, "Fetch all baskets")

	baskets, err := s.basketStore.List(c)
	if err != nil {
		return nil, myerrors.NewInternalError(err)
	}

	sort.Slice(baskets, func(i, j int) bool {
		return baskets[i].CreatedAt.After(baskets[j].CreatedAt)
	})
	return baskets, nil
}

func (s service) createNewBasket(c context.Context, hostname string) (shopmodel.Basket, error) {

	uid := s.uuider.Create()
	createdAt := s.nower.Now()
	returnURL := fmt.Sprintf("%s/basket/%s/checkout/completed", hostname, uid)

	s.logger.Log(c, uid, mylog.SeverityInfo, "Creating new basket with uid %s", uid)

	basket := createBasket(uid, createdAt, returnURL)
	err := s.basketStore.Put(c, uid, basket)
	if err != nil {
		return shopmodel.Basket{}, myerrors.NewInternalError(err)
	}

	return basket, nil
}

func (s service) getBasket(c context.Context, basketUID string) (shopmodel.Basket, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Fetch details of basket uid %s", basketUID)

	basket, found, err := s.basketStore.Get(c, basketUID)
	if err != nil {
		return shopmodel.Basket{}, myerrors.NewInternalError(err)
	}
	if !found {
		return shopmodel.Basket{}, myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID))
	}

	return basket, nil
}

func (s service) checkoutFinalized(c context.Context, basketUID string, status string) (shopmodel.Basket, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Redirect: Checkout finalized for basket %s -> %s", basketUID, status)

	now := s.nower.Now()

	var basket shopmodel.Basket
	var found bool
	var err error
	err = s.basketStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent

		basket, found, err = s.basketStore.Get(c, basketUID)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID))
		}

		basket.InitialPaymentStatus = status
		basket.LastModified = &now

		err = s.basketStore.Put(c, basketUID, basket)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		return nil
	})
	if err != nil {
		return shopmodel.Basket{}, err
	}

	return basket, nil
}

func (s service) checkoutFinalStatusWebhook(c context.Context, basketUID string, eventCode string, status string) error {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Webhook: Checkout status update on basket %s (%s) -> %s", basketUID, eventCode, status)

	now := s.nower.Now()

	var basket shopmodel.Basket
	var found bool
	var err error
	err = s.basketStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent
		basket, found, err = s.basketStore.Get(c, basketUID)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID))
		}

		// Final codes matter!
		basket.FinalPaymentEvent = eventCode
		basket.FinalPaymentStatus = status
		basket.LastModified = &now

		err = s.basketStore.Put(c, basketUID, basket)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func createBasket(uid string, createdAt time.Time, returnURL string) shopmodel.Basket {
	return shopmodel.Basket{
		UID:        uid,
		CreatedAt:  createdAt,
		Shop:       getCurrentShop(),
		Shopper:    getCurrentShopper(uid),
		TotalPrice: 51000,
		Currency:   "EUR",
		SelectedProducts: []shopmodel.SelectedProduct{
			{
				UID:         "product_tennis_racket",
				Description: "Tennis racket",
				Price:       10000,
				Currency:    "EUR",
				Quantity:    5,
			},
			{
				UID:         "product_tennis_balls",
				Description: "Tennis balls",
				Price:       1000,
				Currency:    "EUR",
				Quantity:    1,
			},
		},
		ReturnURL:            returnURL,
		InitialPaymentStatus: "open",
	}
}

func getCurrentShop() shopmodel.Shop {
	return shopmodel.Shop{
		UID:      "shop_evas_shop",
		Name:     "Eva's shop",
		Country:  "NL",
		Currency: "EUR",
		Hostname: "https://www.marcgrolconsultancy.nl/", // "http://localhost:8082"
	}
}

func getCurrentShopper(uid string) shopmodel.Shopper {
	return shopmodel.Shopper{
		UID:         "shopper_marc_grol",
		FirstName:   "Marc",
		LastName:    "Grol",
		DateOfBirth: func() *time.Time { t := time.Date(1971, time.February, 27, 0, 0, 0, 0, time.UTC); return &t }(),
		Address: shopmodel.Address{
			City:              "De Bilt",
			Country:           "NL",
			HouseNumberOrName: "79",
			PostalCode:        "3731TB",
			StateOrProvince:   "Utrecht",
			Street:            "Heemdstrakwartier",
		},
		Country:      "NL",
		Locale:       "nl-NL",
		EmailAddress: fmt.Sprintf("marc.grol+%s@gmail.com", uid),
		PhoneNumber:  "+31648928856",
	}
}
