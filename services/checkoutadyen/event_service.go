package checkoutadyen

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/services/checkoutevents"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthevents"
)

func (s *service) Subscribe(c context.Context) error {
	err := s.subscriber.CreateTopic(c, oauthevents.TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", oauthevents.TopicName, err)
	}

	err = s.subscriber.Subscribe(c, oauthevents.TopicName, myhttp.GuessHostnameWithScheme()+"/api/checkout/event")
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
	// TODO update local store instead of using the shared vault
	return nil
}

func (s *service) OnOAuthTokenRefreshCompleted(c context.Context, topic string, event oauthevents.OAuthTokenRefreshCompleted) error {
	// TODO update local store instead of using the shared vault
	return nil
}
