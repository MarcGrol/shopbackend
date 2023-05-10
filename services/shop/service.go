package shop

import (
	"context"
	"fmt"
	"sort"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/mytime"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/services/checkoutapi"
	"github.com/MarcGrol/shopbackend/services/shop/shopevents"
)

type service struct {
	basketStore mystore.Store[Basket]
	subscriber  mypubsub.PubSub
	publisher   mypublisher.Publisher
	nower       mytime.Nower
	uuider      myuuid.UUIDer
	logger      mylog.Logger
}

// Use dependency injection to isolate the infrastructure and easy testing
func newService(store mystore.Store[Basket], nower mytime.Nower, uuider myuuid.UUIDer, logger mylog.Logger, subscriber mypubsub.PubSub, publisher mypublisher.Publisher) *service {
	return &service{
		basketStore: store,
		subscriber:  subscriber,
		publisher:   publisher,
		nower:       nower,
		uuider:      uuider,
		logger:      logger,
	}
}

func (s *service) CreateTopics(c context.Context) error {
	err := s.publisher.CreateTopic(c, shopevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", shopevents.TopicName, err)
	}
	return nil
}

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

func convertBasketToCheckout(b Basket) checkoutapi.Checkout {
	return checkoutapi.Checkout{
		BasketUID: b.UID,
		ReturnURL: b.ReturnURL,
		TotalAmount: checkoutapi.Amount{
			Value:    b.TotalPrice,
			Currency: b.Currency,
		},
		Company: checkoutapi.Company{
			CountryCode: b.Shop.Country,
			Homepage:    b.Shop.Hostname,
			Name:        b.Shop.Name,
			ShopName:    b.Shop.Name,
		},
		Shopper: checkoutapi.Shopper{
			UID:       b.Shopper.UID,
			Locale:    b.Shopper.Locale,
			FirstName: b.Shopper.FirstName,
			LastName:  b.Shopper.LastName,
			ContactInfo: checkoutapi.ContactInfo{
				Email:       b.Shopper.EmailAddress,
				PhoneNumber: b.Shopper.PhoneNumber,
			},
			Address: checkoutapi.Address{
				City:               b.Shopper.Address.City,
				Country:            b.Shopper.Address.Country,
				AddressHouseNumber: b.Shopper.Address.HouseNumberOrName,
				PostalCode:         b.Shopper.Address.PostalCode,
				State:              b.Shopper.Address.StateOrProvince,
				Street:             b.Shopper.Address.Street,
			},
		},
		Products: func() []checkoutapi.Product {
			products := []checkoutapi.Product{}
			for _, p := range b.SelectedProducts {
				products = append(products, checkoutapi.Product{
					Name:        p.UID,
					Description: p.Description,
					ItemPrice:   p.Price,
					Currency:    b.Currency,
					Quantity:    p.Quantity,
					TotalPrice:  p.Quantity * p.Price,
				})
			}
			return products
		}(),
	}
}
