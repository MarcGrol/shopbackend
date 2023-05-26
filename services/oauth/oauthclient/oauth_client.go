package oauthclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient/challenge"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
)

type ComposeAuthURLRequest struct {
	ProviderName  string
	CompletionURL string
	Scope         string
	State         string
}

type GetTokenRequest struct {
	ProviderName string
	RedirectURI  string
	Code         string
	CodeVerifier string
}

type RefreshTokenRequest struct {
	ProviderName string `json:"-"`
	RefreshToken string `json:"refresh_token"`
}

type CancelTokenRequest struct {
	ProviderName string `json:"-"`
	AccessToken  string `json:"access_token"`
}

type GetTokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

//go:generate mockgen -source=oauth_client.go -package oauthclient -destination oauth_client_mock.go OauthClient
type OauthClient interface {
	ComposeAuthURL(c context.Context, req ComposeAuthURLRequest) (string, string, error)
	GetAccessToken(c context.Context, req GetTokenRequest) (GetTokenResponse, error)
	RefreshAccessToken(c context.Context, req RefreshTokenRequest) (GetTokenResponse, error)
	CancelAccessToken(c context.Context, req CancelTokenRequest) error
}

type oauthClient struct {
	providers      *providers.OAuthProviders
	randomStringer challenge.RandomStringer
}

func NewOAuthClient(providers *providers.OAuthProviders, randomStringer challenge.RandomStringer) *oauthClient {
	return &oauthClient{
		providers:      providers,
		randomStringer: randomStringer,
	}
}

func (oc oauthClient) ComposeAuthURL(c context.Context, req ComposeAuthURLRequest) (string, string, error) {
	provider, err := oc.providers.Get(req.ProviderName)
	if err != nil {
		return "", "", fmt.Errorf("provider with name %s not known", req.ProviderName)
	}

	authURL := provider.AuthEndpoint.GetFullURL()
	u, err := url.Parse(authURL)
	if err != nil {
		return "", "", err
	}

	randomString, err := oc.randomStringer.Create()
	if err != nil {
		return "", "", myerrors.NewInternalError(fmt.Errorf("error creating seed: %s", err))
	}

	method, challenge, err := challenge.Create(randomString)
	if err != nil {
		return "", "", err
	}

	u.RawQuery = url.Values{
		"client_id":             []string{provider.ClientID},
		"code_challenge":        []string{challenge},
		"code_challenge_method": []string{method},
		"redirect_uri":          []string{req.CompletionURL},
		"response_type":         []string{"code"},
		"scope":                 []string{req.Scope},
		"state":                 []string{req.State},
	}.Encode()

	return u.String(), randomString, nil
}

func (oc oauthClient) GetAccessToken(c context.Context, req GetTokenRequest) (GetTokenResponse, error) {
	provider, err := oc.providers.Get(req.ProviderName)
	if err != nil {
		return GetTokenResponse{}, fmt.Errorf("provider with name '%s' not known", req.ProviderName)
	}

	clientID, secret := provider.GetCredentials(provider)

	getTokenURL := provider.TokenEndpoint.GetFullURL()

	requestBody := url.Values{
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {req.RedirectURI},
		"code":          {req.Code},
		"code_verifier": {req.CodeVerifier},
	}.Encode()

	httpClient := newHTTPClient(clientID, secret)
	httpRespCode, respBody, err := httpClient.Send(c, http.MethodPost, getTokenURL, []byte(requestBody))
	if err != nil {
		return GetTokenResponse{}, fmt.Errorf("error getting token: %s", err)
	}

	if httpRespCode != 200 {
		return GetTokenResponse{}, fmt.Errorf("error getting token: %d", httpRespCode)
	}

	resp := GetTokenResponse{}
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		return GetTokenResponse{}, fmt.Errorf("error parsing response: %s", err)
	}

	return resp, nil
}

func (oc oauthClient) RefreshAccessToken(c context.Context, req RefreshTokenRequest) (GetTokenResponse, error) {
	provider, err := oc.providers.Get(req.ProviderName)
	if err != nil {
		return GetTokenResponse{}, fmt.Errorf("provider with name '%s' not known", req.ProviderName)
	}

	clientID, secret := provider.GetCredentials(provider)

	refreshTokenURL := provider.TokenEndpoint.GetFullURL()

	requestBody := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {req.RefreshToken},
	}.Encode()

	httpClient := newHTTPClient(clientID, secret)
	httpRespCode, respBody, err := httpClient.Send(c, http.MethodPost, refreshTokenURL, []byte(requestBody))
	if err != nil {
		return GetTokenResponse{}, fmt.Errorf("error getting refresh-token: %s", err)
	}

	if httpRespCode != 200 {
		return GetTokenResponse{}, fmt.Errorf("error getting refresh-token: %d", httpRespCode)
	}

	resp := GetTokenResponse{}
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		return GetTokenResponse{}, fmt.Errorf("error parsing response: %s", err)
	}

	return resp, nil
}

func (oc oauthClient) CancelAccessToken(c context.Context, req CancelTokenRequest) error {
	// TODO: implement
	return nil
}
