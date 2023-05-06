package shop

import (
	"context"
	"fmt"
	"github.com/MarcGrol/shopbackend/lib/myhttp"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/services/checkout/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
	"github.com/MarcGrol/shopbackend/services/shop/shopevents"
)

func (s *service) Subscribe(c context.Context) error {

	err := s.pubsub.CreateTopic(c, shopevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", oauthevents.TopicName, err)
	}

	err = s.pubsub.Subscribe(c, checkoutevents.TopicName, myhttp.GuessHostnameWithScheme()+"/basket/event")
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

		if basket.Done {
			return nil
		}

		basket.FinalPaymentEvent = event.Status
		basket.FinalPaymentStatus = event.Success
		basket.LastModified = &now
		basket.PaymentMethod = event.PaymentMethod
		basket.Done = true

		err = s.basketStore.Put(c, event.CheckoutUID, basket)
		if err != nil {
			return myerrors.NewInternalError(err)
		}

		err = s.publisher.Publish(c, shopevents.TopicName, shopevents.BasketPaymentCompleted{
			BasketUID: event.CheckoutUID},
		)
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
