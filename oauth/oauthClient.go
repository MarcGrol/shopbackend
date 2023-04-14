package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/MarcGrol/shopbackend/lib/codeverifier"
)

type ComposeAuthURLRequest struct {
	CompletionURL string
	Scope         string
	State         string
	CodeVerifier  string
}

type GetTokenRequest struct {
	RedirectUri  string
	Code         string
	CodeVerifier string
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type GetTokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	Scope        string `json:"exampleScope"`
	RefreshToken string `json:"refresh_token"`
}

//go:generate mockgen -source=oauthClient.go -package oauth -destination oauthClient_mock.go OauthClient
type OauthClient interface {
	ComposeAuthURL(c context.Context, req ComposeAuthURLRequest) (string, error)
	GetAccessToken(c context.Context, req GetTokenRequest) (GetTokenResponse, error)
	RefreshAccessToken(c context.Context, req RefreshTokenRequest) (GetTokenResponse, error)
}

type oauthClient struct {
	clientID      string
	clientSecret  string
	authHostname  string
	tokenHostname string
}

func NewOAuthClient(clientId string, clientSecret string, authHostname string, tokenHostname string) OauthClient {
	return &oauthClient{
		clientID:      clientId,
		clientSecret:  clientSecret,
		authHostname:  authHostname,
		tokenHostname: tokenHostname,
	}
}

const (
	authURL  = "/ca/ca/oauth/connect.shtml"
	tokenURL = "/v1/token"
)

func (g oauthClient) ComposeAuthURL(c context.Context, req ComposeAuthURLRequest) (string, error) {
	u, err := url.Parse(g.authHostname + authURL)
	if err != nil {
		return "", err
	}

	method, challenge := codeverifier.NewVerifierFrom(req.CodeVerifier).CreateChallenge()

	u.RawQuery = url.Values{
		"client_id":             []string{g.clientID},
		"code_challenge":        []string{challenge},
		"code_challenge_method": []string{method},
		"redirect_uri":          []string{req.CompletionURL},
		"response_type":         []string{"code"},
		"scope":                 []string{req.Scope},
		"state":                 []string{req.State},
	}.Encode()

	return u.String(), nil
}

func (g oauthClient) GetAccessToken(ctx context.Context, req GetTokenRequest) (GetTokenResponse, error) {
	requestBody := url.Values{
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {req.RedirectUri},
		"code":          {req.Code},
		"code_verifier": {req.CodeVerifier},
	}.Encode()

	httpClient := newHttpClient(g.clientID, g.clientSecret)
	httpRespCode, respBody, err := httpClient.Send(ctx, http.MethodPost, g.tokenHostname+tokenURL, []byte(requestBody))
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

func (g oauthClient) RefreshAccessToken(ctx context.Context, req RefreshTokenRequest) (GetTokenResponse, error) {
	requestBody := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {req.RefreshToken},
	}.Encode()

	httpClient := newHttpClient(g.clientID, g.clientSecret)
	httpRespCode, respBody, err := httpClient.Send(ctx, http.MethodPost, g.tokenHostname+tokenURL, []byte(requestBody))
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
