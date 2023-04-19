package shop

import (
	"context"
	"fmt"
	"github.com/MarcGrol/shopbackend/services/shop/shopevents"
	"sort"
	"time"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/services/checkout/checkoutevents"
)

func (s *service) listBaskets(c context.Context) ([]Basket, error) {
	s.logger.Log(c, "", mylog.SeverityInfo, "Fetch all baskets")

	baskets, err := s.basketStore.List(c)
	if err != nil {
		return nil, myerrors.NewInternalError(err)
	}

	// TODO sort in database
	sort.Slice(baskets, func(i, j int) bool {
		return baskets[i].CreatedAt.After(baskets[j].CreatedAt)
	})
	return baskets, nil
}

func (s *service) createNewBasket(c context.Context, hostname string) (Basket, error) {

	basketUID := s.uuider.Create()
	createdAt := s.nower.Now()
	returnURL := fmt.Sprintf("%s/basket/%s/checkout/completed", hostname, basketUID)
	basket := createBasket(basketUID, createdAt, returnURL)

	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Creating new basket with uid %s", basketUID)

	err := s.basketStore.RunInTransaction(c, func(c context.Context) error {
		err := s.basketStore.Put(c, basketUID, basket)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		err = s.publisher.Publish(c, shopevents.TopicName, shopevents.BasketCreated{
			BasketUID: basketUID},
		)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		return nil
	})
	if err != nil {
		return Basket{}, err
	}

	return basket, nil
}

func (s service) getBasket(c context.Context, basketUID string) (Basket, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Fetch details of basket uid %s", basketUID)

	basket, found, err := s.basketStore.Get(c, basketUID)
	if err != nil {
		return Basket{}, myerrors.NewInternalError(err)
	}
	if !found {
		return Basket{}, myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", basketUID))
	}

	return basket, nil
}

func (s *service) checkoutFinalized(c context.Context, basketUID string, status string) (Basket, error) {
	s.logger.Log(c, basketUID, mylog.SeverityInfo, "Redirect: Checkout finalized for basket %s -> %s", basketUID, status)

	now := s.nower.Now()

	var basket Basket
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
		return Basket{}, err
	}

	return basket, nil
}

func (s *service) handleCheckoutCompletedEvent(c context.Context, event checkoutevents.CheckoutCompleted) error {
	s.logger.Log(c, event.CheckoutUID, mylog.SeverityInfo, "Webhook: Checkout status update on basket %s (%s) -> %v", event.CheckoutUID, event.Status, event.Status)

	now := s.nower.Now()

	err := s.basketStore.RunInTransaction(c, func(c context.Context) error {
		// must be idempotent
		basket, found, err := s.basketStore.Get(c, event.CheckoutUID)
		if err != nil {
			return myerrors.NewInternalError(err)
		}
		if !found {
			return myerrors.NewNotFoundError(fmt.Errorf("basket with uid %s not found", event.CheckoutUID))
		}

		// Final codes matter!
		basket.FinalPaymentEvent = event.Status
		basket.FinalPaymentStatus = event.Success
		basket.LastModified = &now
		basket.PaymentMethod = event.PaymentMethod

		err = s.basketStore.Put(c, event.CheckoutUID, basket)
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

func createBasket(uid string, createdAt time.Time, returnURL string) Basket {
	return Basket{
		UID:        uid,
		CreatedAt:  createdAt,
		Shop:       getCurrentShop(),
		Shopper:    getCurrentShopper(uid),
		TotalPrice: 51000,
		Currency:   "EUR",
		SelectedProducts: []SelectedProduct{
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

func getCurrentShop() Shop {
	return Shop{
		UID:      "shop_evas_shop",
		Name:     "Eva's shop",
		Country:  "NL",
		Currency: "EUR",
		Hostname: "https://www.marcgrolconsultancy.nl/", // "http://localhost:8082"
	}
}

func getCurrentShopper(uid string) Shopper {
	return Shopper{
		UID:         "shopper_marc_grol",
		FirstName:   "Marc",
		LastName:    "Grol",
		DateOfBirth: func() *time.Time { t := time.Date(1971, time.February, 27, 0, 0, 0, 0, time.UTC); return &t }(),
		Address: Address{
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
