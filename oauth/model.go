package oauth

type Session struct {
	UID       string
	ReturnURL string
	Verifier  string
	TokenData GetTokenResponse
}
