package oauthvault

import "time"

const (
	CurrentToken = "currentToken"
)

type Token struct {
	ProviderName string
	ClientID     string
	SessionUID   string
	Scopes       string
	CreatedAt    time.Time
	LastModified *time.Time
	AccessToken  string
	RefreshToken string
	ExpiresIn    *time.Time
}
