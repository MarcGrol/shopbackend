package oauthevents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myevents"
)

const (
	TopicName                       = "oauth"
	oauthSessionSetupStartedName    = TopicName + ".sessionSetup.started"
	oauthSessionSetupCompletedName  = TopicName + ".sessionSetup.completed"
	oauthTokenCreationCompletedName = TopicName + ".tokenCreation.completed"
	oauthTokenRefreshCompletedName  = TopicName + ".tokenRefresh.completed"
)

type OAuthEventService interface {
	Subscribe(c context.Context) error
	OnOAuthSessionSetupStarted(c context.Context, topic string, event OAuthSessionSetupStarted) error
	OnOAuthSessionSetupCompleted(c context.Context, topic string, event OAuthSessionSetupCompleted) error
	OnOAuthTokenCreationCompleted(c context.Context, topic string, event OAuthTokenCreationCompleted) error
	OnOAuthTokenRefreshCompleted(c context.Context, topic string, event OAuthTokenRefreshCompleted) error
}

func DispatchEvent(c context.Context, reader io.Reader, service OAuthEventService) error {
	envelope, err := myevents.ParseEventEnvelope(reader)
	if err != nil {
		return myerrors.NewInvalidInputError(err)
	}

	switch envelope.EventTypeName {
	case oauthSessionSetupStartedName:
		{
			event := OAuthSessionSetupStarted{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnOAuthSessionSetupStarted(c, envelope.Topic, event)
		}
	case oauthSessionSetupCompletedName:
		{
			event := OAuthSessionSetupCompleted{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnOAuthSessionSetupCompleted(c, envelope.Topic, event)
		}
	case oauthTokenCreationCompletedName:
		{
			event := OAuthTokenCreationCompleted{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnOAuthTokenCreationCompleted(c, envelope.Topic, event)
		}
	case oauthTokenRefreshCompletedName:
		{
			event := OAuthTokenRefreshCompleted{}
			err := json.Unmarshal([]byte(envelope.EventPayload), &event)
			if err != nil {
				return myerrors.NewInvalidInputError(err)
			}
			return service.OnOAuthTokenRefreshCompleted(c, envelope.Topic, event)
		}
	default:
		return myerrors.NewNotImplementedError(fmt.Errorf(envelope.EventTypeName))
	}
}

type OAuthSessionSetupStarted struct {
	ProviderName string
	ClientID     string
	SessionUID   string
	Scopes       string
}

func (e OAuthSessionSetupStarted) GetEventTypeName() string {
	return oauthSessionSetupStartedName
}

func (e OAuthSessionSetupStarted) GetAggregateName() string {
	return e.ClientID
}

type OAuthSessionSetupCompleted struct {
	ProviderName string
	ClientID     string
	SessionUID   string
	Success      bool
	ErrorMessage string
}

func (e OAuthSessionSetupCompleted) GetEventTypeName() string {
	return oauthSessionSetupCompletedName
}

func (e OAuthSessionSetupCompleted) GetAggregateName() string {
	return e.ClientID
}

type OAuthTokenCreationCompleted struct {
	ProviderName string
	ClientID     string
	SessionUID   string
	Success      bool
	ErrorMessage string
}

func (e OAuthTokenCreationCompleted) GetEventTypeName() string {
	return oauthTokenCreationCompletedName
}

func (e OAuthTokenCreationCompleted) GetAggregateName() string {
	return e.ClientID
}

type OAuthTokenRefreshCompleted struct {
	ProviderName string
	UID          string
	ClientID     string
	Success      bool
	ErrorMessage string
}

func (e OAuthTokenRefreshCompleted) GetEventTypeName() string {
	return oauthTokenRefreshCompletedName
}

func (e OAuthTokenRefreshCompleted) GetAggregateName() string {
	return e.ClientID
}

type OAuthTokenRefreshFailed struct {
	ProviderName string
	UID          string
	ClientID     string
	Success      bool
	ErrorMessage string
}
