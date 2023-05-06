package oauth

import "fmt"

type EndPoint struct {
	Hostname string
	Path     string
}

func (ep EndPoint) GetFullURL() string {
	return ep.Hostname + ep.Path
}

type OauthProvider struct {
	ClientID      string
	Secret        string
	AuthEndpoint  EndPoint
	TokenEndpoint EndPoint
}

type OauthProviders map[string]OauthProvider

var (
	oauthProviders = OauthProviders{
		"adyen": {
			AuthEndpoint: EndPoint{
				Hostname: "https://ca-test.adyen.com",
				Path:     "/ca/ca/oauth/connect.shtml",
			},
			TokenEndpoint: EndPoint{
				Hostname: "https://oauth-test.adyen.com",
				Path:     "/v1/token",
			},
		},
		"stripe": {
			AuthEndpoint: EndPoint{
				Hostname: "https://connect.stripe.com",
				Path:     "/oauth/authorize",
			},
			TokenEndpoint: EndPoint{
				Hostname: "https://connect.stripe.com",
				Path:     "/oauth/token",
			},
		},
	}
)

func ConfigureProvider(providerName string, clientID string, secret string, authHostname string, tokenHostname string) (OauthProviders, error) {
	provider, found := oauthProviders[providerName]
	if !found {
		return nil, fmt.Errorf("Oauth provider with name '%s' not found", providerName)
	}

	provider.ClientID = clientID
	provider.Secret = secret
	if authHostname != "" {
		provider.AuthEndpoint.Hostname = authHostname
	}
	if tokenHostname != "" {
		provider.TokenEndpoint.Hostname = tokenHostname
	}

	oauthProviders[providerName] = provider

	return oauthProviders, nil
}

func GetProviderDetails(providerName string) (OauthProvider, error) {
	provider, found := oauthProviders[providerName]
	if !found {
		return OauthProvider{}, fmt.Errorf("Oauth provider with name '%s' not found", providerName)
	}
	return provider, nil
}
