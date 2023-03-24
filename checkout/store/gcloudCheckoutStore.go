package store

import (
	"context"
	"os"

	"github.com/MarcGrol/shopbackend/checkout/checkoutmodel"
	"github.com/MarcGrol/shopbackend/mystore"
)

type gcloudPaymentStore struct {
	gcloudDatastoreClient mystore.DataStorer
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		New = NewGcloudCheckoutStore
	}
}

func NewGcloudCheckoutStore(c context.Context) (CheckoutStorer, func(), error) {
	store, cleanup, err := mystore.NewStore(c)
	if err != nil {
		return nil, func() {}, err
	}
	return &gcloudPaymentStore{
		gcloudDatastoreClient: store,
	}, cleanup, nil
}

func (s *gcloudPaymentStore) Put(ctx context.Context, basketUID string, paymentData *checkoutmodel.CheckoutContext) error {
	return s.gcloudDatastoreClient.Put(ctx, "CheckoutContext", basketUID, paymentData)
}

func (s *gcloudPaymentStore) Get(ctx context.Context, basketUID string) (checkoutmodel.CheckoutContext, bool, error) {
	checkout := checkoutmodel.CheckoutContext{}
	exists, err := s.gcloudDatastoreClient.Get(ctx, "CheckoutContext", basketUID, &checkout)
	if err != nil {
		return checkout, false, err
	}
	return checkout, exists, nil
}

func (s *gcloudPaymentStore) List(ctx context.Context) ([]checkoutmodel.CheckoutContext, error) {
	checkouts := []checkoutmodel.CheckoutContext{}
	err := s.gcloudDatastoreClient.List(ctx, "CheckoutContext", &checkouts)
	if err != nil {
		return checkouts, err
	}
	return checkouts, nil
}
