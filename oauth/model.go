package oauth

import "time"

type OAuthSessionSetup struct {
	UID          string
	ClientID     string
	Scopes       string
	ReturnURL    string
	Verifier     string
	CreatedAt    time.Time
	LastModified *time.Time
	TokenData    *GetTokenResponse
	Done         bool
}
