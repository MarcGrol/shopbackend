package providers

import "fmt"

type EndPoint struct {
	Hostname string
	Path     string
}

func (ep EndPoint) GetFullURL() string {
	return ep.Hostname + ep.Path
}

type OauthParty struct {
	ClientID       string
	Secret         string
	AuthEndpoint   EndPoint
	TokenEndpoint  EndPoint
	DefaultScopes  string
	GetCredentials func(p OauthParty) (string, string)
}

type OAuthProvider interface {
	All() map[string]OauthParty
	Set(providerName string, clientID string, secret string, authHostname string, tokenHostname string)
	Get(providerName string) (OauthParty, error)
}

type OAuthProviders struct {
	providers map[string]OauthParty
}

func NewProviders() *OAuthProviders {
	return &OAuthProviders{
		providers: map[string]OauthParty{
			"adyen": {
				ClientID: "adyen_client_id",
				Secret:   "adyen_secret",
				AuthEndpoint: EndPoint{
					Hostname: "https://ca-test.adyen.com",
					Path:     "/ca/ca/oauth/connect.shtml",
				},
				TokenEndpoint: EndPoint{
					Hostname: "https://oauth-test.adyen.com",
					Path:     "/v1/token",
				},
				DefaultScopes: "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write psp:paybylink:write",
				GetCredentials: func(p OauthParty) (string, string) {
					return p.ClientID, p.Secret
				},
			},
			"stripe": {
				ClientID: "stripe_client_id",
				Secret:   "stripe_secret",
				AuthEndpoint: EndPoint{
					Hostname: "https://connect.stripe.com",
					Path:     "/oauth/authorize",
				},
				TokenEndpoint: EndPoint{
					Hostname: "https://connect.stripe.com",
					Path:     "/oauth/token",
				},
				DefaultScopes: "read_write",
				GetCredentials: func(p OauthParty) (string, string) {
					return p.Secret, "" // secret is used as basic auth username with empty password
				},
			},
		},
	}
}

func (op *OAuthProviders) All() map[string]OauthParty {
	return op.providers
}

func (op *OAuthProviders) Set(providerName string, clientID string, secret string, authHostname string, tokenHostname string) {
	provider, found := op.providers[providerName]
	if !found {
		provider = OauthParty{}
	}

	if clientID != "" {
		provider.ClientID = clientID
	}

	if secret != "" {
		provider.Secret = secret
	}

	if authHostname != "" {
		provider.AuthEndpoint.Hostname = authHostname
	}

	if tokenHostname != "" {
		provider.TokenEndpoint.Hostname = tokenHostname
	}

	op.providers[providerName] = provider
}

func (op *OAuthProviders) Get(providerName string) (OauthParty, error) {
	provider, found := op.providers[providerName]
	if !found {
		return OauthParty{}, fmt.Errorf("oauth provider with name '%s' not found", providerName)
	}
	return provider, nil
}
