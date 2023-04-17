package oauthevents

const (
	TopicName                       = "oauth"
	OAuthSessionSetupStartedName    = TopicName + ".sessionSetup.started"
	OAuthSessionSetupCompletedName  = TopicName + ".sessionSetup.completed"
	OAuthTokenCreationCompletedName = TopicName + ".tokenCreation.completed"
	OAuthTokenRefreshCompletedName  = TopicName + ".tokenRefresh.completed"
)

type OAuthSessionSetupStarted struct {
	ClientID   string
	SessionUID string
	Scopes     string
}

func (e OAuthSessionSetupStarted) GetEventTypeName() string {
	return OAuthSessionSetupStartedName
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
	return OAuthSessionSetupCompletedName
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
	return OAuthTokenCreationCompletedName
}

func (e OAuthTokenCreationCompleted) GetAggregateName() string {
	return e.ClientID
}

type OAuthTokenRefreshCompleted struct {
	UID          string
	ClientID     string
	Success      bool
	ErrorMessage string
}

func (e OAuthTokenRefreshCompleted) GetEventTypeName() string {
	return OAuthTokenRefreshCompletedName
}

func (e OAuthTokenRefreshCompleted) GetAggregateName() string {
	return e.ClientID
}
