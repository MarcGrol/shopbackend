package oauth

const (
	TopicName = "oauth"
)

type OAuthSessionSetupStarted struct {
	SessionUID string
	ClientID   string
}

func (OAuthSessionSetupStarted) GetEventTypeName() string {
	return "oauth.sessionSetup.started"
}

type OAuthSessionSetupCompleted struct {
	SessionUID   string
	Success      bool
	ErrorMessage string
}

func (OAuthSessionSetupCompleted) GetEventTypeName() string {
	return "oauth.sessionSetup.completed"
}

type OAuthTokenCreationCompleted struct {
	SessionUID   string
	Success      bool
	ErrorMessage string
}

func (OAuthTokenCreationCompleted) GetEventTypeName() string {
	return "oauth.tokenCreation.completed"
}

type OAuthTokenRefreshCompleted struct {
	SessionUID   string
	Success      bool
	ErrorMessage string
}

func (OAuthTokenRefreshCompleted) GetEventTypeName() string {
	return "oauth.tokenRefresh.completed"
}
