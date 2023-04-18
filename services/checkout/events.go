package checkout

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/mypubsub"
	"github.com/MarcGrol/shopbackend/services/checkout/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
)

func (s *service) Subscribe(c context.Context) error {
	client, cleanup, err := mypubsub.New(c)
	if err != nil {
		return fmt.Errorf("error creating client: %s", err)
	}
	defer cleanup()

	err = client.Subscribe(c, oauthevents.TopicName, "https://www.marcgrolconsultancy.nl/checkout/event")
	if err != nil {
		return fmt.Errorf("error subscribing to topic %s: %s", checkoutevents.TopicName, err)
	}

	return nil
}

func (s *service) OnOAuthSessionSetupStarted(c context.Context, topic string, event oauthevents.OAuthSessionSetupStarted) error {
	return nil
}

func (s *service) OnOAuthSessionSetupCompleted(c context.Context, topic string, event oauthevents.OAuthSessionSetupCompleted) error {
	return nil
}

func (s *service) OnOAuthTokenCreationCompleted(c context.Context, topic string, event oauthevents.OAuthTokenCreationCompleted) error {
	return nil
}

func (s *service) OnOAuthTokenRefreshCompleted(c context.Context, topic string, event oauthevents.OAuthTokenRefreshCompleted) error {
	return nil
}
