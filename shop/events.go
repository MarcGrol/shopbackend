package shop

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/checkout/checkoutevents"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/oauth/oauthevents"
)

func (s *service) Subscribe(c context.Context) error {
	client, cleanup, err := mypubsub.New(c)
	if err != nil {
		return fmt.Errorf("error creating client: %s", err)
	}
	defer cleanup()

	err = client.CreateTopic(c, checkoutevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", checkoutevents.TopicName, err)
	}

	err = client.Subscribe(c, oauthevents.TopicName, "https://www.marcgrolconsultancy.nl/basket/event")
	if err != nil {
		return fmt.Errorf("error subscribing to topic %s: %s", checkoutevents.TopicName, err)
	}

	return nil
}

func (s *service) OnCheckoutStarted(c context.Context, topic string, event checkoutevents.CheckoutStarted) error {
	return nil
}

func (s *service) OnCheckoutCompleted(c context.Context, topic string, event checkoutevents.CheckoutCompleted) error {
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
