package oauth

import "time"

type Session struct {
	UID          string
	ReturnURL    string
	Verifier     string
	CreatedAt    time.Time
	LastModified *time.Time
	TokenData    GetTokenResponse
}
