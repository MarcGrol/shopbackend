package oauth

import "time"

type OAuthSessionSetup struct {
	UID          string
	ProviderName string
	ClientID     string
	Scopes       string
	ReturnURL    string
	Verifier     string
	CreatedAt    time.Time
	LastModified *time.Time
	TokenData    *GetTokenResponse
	Done         bool
}

type OAuthStatus struct {
	ProviderName string
	ClientID     string
	SessionUID   string
	Scopes       string
	CreatedAt    time.Time
	LastModified *time.Time
	ValidUntil   time.Time
	Status       bool
}
