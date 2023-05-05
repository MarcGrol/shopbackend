package oauth

type oauthService struct {
	AuthURL  string
	TokenURL string
}

var (
	servers = map[string]oauthService{
		"adyen": {
			AuthURL:  "/ca/ca/oauth/connect.shtml",
			TokenURL: "/v1/token",
		},
		"stripe": {
			AuthURL:  "/oauth/authorize",
			TokenURL: "/oauth/token",
		},
	}
)
