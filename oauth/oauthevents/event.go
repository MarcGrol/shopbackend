package oauthevents

const (
	TopicName = "oauth"
)

type OAuthSessionSetupStarted struct {
	ClientID   string
	SessionUID string
	Scopes     string
}

func (e OAuthSessionSetupStarted) GetEventTypeName() string {
	return "oauth.sessionSetup.started"
}

func (e OAuthSessionSetupStarted) GetAggregateName() string {
	return e.ClientID
}

type OAuthSessionSetupCompleted struct {
	ClientID     string
	SessionUID   string
	Success      bool
	ErrorMessage string
}

func (e OAuthSessionSetupCompleted) GetEventTypeName() string {
	return "oauth.sessionSetup.completed"
}

func (e OAuthSessionSetupCompleted) GetAggregateName() string {
	return e.ClientID
}

type OAuthTokenCreationCompleted struct {
	ClientID     string
	SessionUID   string
	Success      bool
	ErrorMessage string
}

func (e OAuthTokenCreationCompleted) GetEventTypeName() string {
	return "oauth.tokenCreation.completed"
}

func (e OAuthTokenCreationCompleted) GetAggregateName() string {
	return e.ClientID
}

type OAuthTokenRefreshCompleted struct {
	ClientID     string
	Success      bool
	ErrorMessage string
}

func (e OAuthTokenRefreshCompleted) GetEventTypeName() string {
	return "oauth.tokenRefresh.completed"
}

func (e OAuthTokenRefreshCompleted) GetAggregateName() string {
	return e.ClientID
}
