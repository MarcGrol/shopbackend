package shop

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/myhttp"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/shop/shopevents"
)

func (s *service) Subscribe(c context.Context) error {
	err := s.subscriber.CreateTopic(c, checkoutevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", checkoutevents.TopicName, err)
	}

	err = s.subscriber.Subscribe(c, checkoutevents.TopicName, myhttp.GuessHostnameWithScheme()+"/api/basket/event")
	if err != nil {
		return fmt.Errorf("error subscribing to topic %s: %s", checkoutevents.TopicName, err)
	}

	return nil
}

func (s *service) OnCheckoutStarted(c context.Context, topic string, event checkoutevents.CheckoutStarted) error {
	s.logger.Log(c, event.CheckoutUID, mylog.SeverityInfo, "Webhook: Checkout started for basket %s using psp %s", event.CheckoutUID, event.ProviderName)

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

		basket.PaymentServiceProvider = event.ProviderName

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

func (s *service) OnCheckoutCompleted(c context.Context, topic string, event checkoutevents.CheckoutCompleted) error {
	s.logger.Log(c, event.CheckoutUID, mylog.SeverityInfo, "Webhook: Checkout status update on basket %s: %v (%s)", event.CheckoutUID, event.CheckoutStatus, event.CheckoutStatusDetails)

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

		basket.PaymentServiceProvider = event.ProviderName
		basket.LastModified = &now
		basket.PaymentMethod = event.PaymentMethod
		basket.Done = true
		basket.CheckoutStatus = string(event.CheckoutStatus)
		basket.CheckoutStatusDetails = event.CheckoutStatusDetails

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

func (s *service) OnPayByLinkCreated(c context.Context, topic string, event checkoutevents.PayByLinkCreated) error {
	s.logger.Log(c, event.CheckoutUID, mylog.SeverityInfo, "Webhook: PaybyLink created on basket %s: %v", event.CheckoutUID, event)

	return nil
}
