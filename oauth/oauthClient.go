package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/MarcGrol/shopbackend/lib/codeverifier"
	"net/http"
	"net/url"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
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

type GetTokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	Scope        string `json:"exampleScope"`
	RefreshToken string `json:"refresh_token"`
}

type OauthClient interface {
	ComposeAuthURL(c context.Context, req ComposeAuthURLRequest) (string, error)
	GetAccessToken(c context.Context, req GetTokenRequest) (GetTokenResponse, error)
}

type oauthClient struct {
	clientID     string
	clientSecret string
}

func NewOAuthClient(clientId string, clientSecret string) OauthClient {
	return &oauthClient{
		clientID:     clientId,
		clientSecret: clientSecret,
	}
}

const (
	authURL  = "https://www.oauth.com/playground/auth-dialog.html"
	tokenURL = "https://www.oauth.com/playground/token-dialog.html"
)

func (g oauthClient) ComposeAuthURL(c context.Context, req ComposeAuthURLRequest) (string, error) {
	u, err := url.Parse(authURL)
	if err != nil {
		return "", err
	}

	challenge, method := codeverifier.NewVerifierFrom(req.CodeVerifier).CreateChallenge()

	u.RawQuery = url.Values{
		"response_type":         []string{"code"},
		"client_id":             []string{g.clientID},
		"redirect_uri":          []string{req.CompletionURL},
		"exampleScope":          []string{req.Scope},
		"state":                 []string{req.State},
		"code_challenge_method": []string{method},
		"code_challenge":        []string{challenge},
	}.Encode()

	return u.String(), nil
}

func (g oauthClient) GetAccessToken(ctx context.Context, req GetTokenRequest) (GetTokenResponse, error) {
	requestBody := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {g.clientID},
		"redirect_uri":  {req.RedirectUri},
		"code":          {req.Code},
		"code_verifier": {req.CodeVerifier},
	}.Encode()

	httpClient := newHttpClient(g.clientID, g.clientSecret)
	httpRespCode, respBody, err := httpClient.Send(ctx, http.MethodPost, tokenURL, []byte(requestBody))
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

func (g oauthClient) GetRefreshToken(c context.Context, tokenURL string, code string, returnURL string) (GetTokenResponse, error) {
	return GetTokenResponse{}, myerrors.NewNotImplementedError(fmt.Errorf("Refresh token"))
}
