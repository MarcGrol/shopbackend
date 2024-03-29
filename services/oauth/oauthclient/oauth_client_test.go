package oauthclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"

	"github.com/MarcGrol/shopbackend/services/oauth/oauthclient/challenge"
	"github.com/MarcGrol/shopbackend/services/oauth/providers"
)

const (
	adyenExampleScopes = "psp.onlinepayment:write psp.accountsettings:write psp.webhook:write"
)

func RunGetAccessTokenServer(t *testing.T,
	unserializer func(t *testing.T, r *http.Request) GetTokenRequest,
	verifier func(t *testing.T, r *http.Request, req GetTokenRequest),
	responder func(t *testing.T, req GetTokenRequest) GetTokenResponse,
	serializer func(t *testing.T, resp GetTokenResponse, w http.ResponseWriter)) (*httptest.Server, func()) {

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	providers := providers.NewProviders()
	adyenProvider, err := providers.Get("adyen")
	assert.NoError(t, err)

	mux.HandleFunc(adyenProvider.TokenEndpoint.Path, func(w http.ResponseWriter, r *http.Request) {
		req := unserializer(t, r)
		verifier(t, r, req)
		resp := responder(t, req)
		serializer(t, resp, w)
	})
	return ts, func() {
		defer ts.Close()
	}
}

func TestOAuthClient(t *testing.T) {
	t.Run("Compose auth url", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		randomStringer := challenge.NewMockRandomStringer(ctrl)
		randomStringer.EXPECT().Create().Return("05796efe18af079dc654bb88c68f5cd8b8a5d378e7cec8e9856258f95d3b0b5a", nil)

		providers := providers.NewProviders()
		providers.Set("adyen", "123", "456", "https://ca-test.adyen.com", "https://oauth-test.adyen.com")

		oauthClient := NewOAuthClient(providers, randomStringer)
		url, randomString, err := oauthClient.ComposeAuthURL(context.TODO(), ComposeAuthURLRequest{
			ProviderName:  "adyen",
			CompletionURL: "http://localhost:8888/oauth/done",
			Scope:         adyenExampleScopes,
			State:         "abcdef",
		})
		assert.NoError(t, err)
		assert.Equal(t, "05796efe18af079dc654bb88c68f5cd8b8a5d378e7cec8e9856258f95d3b0b5a", randomString)
		assert.Equal(t, "https://ca-test.adyen.com/ca/ca/oauth/connect.shtml?client_id=123&code_challenge=A-Y4cHhqIJi48r-V_cKdDRzlMJmC8zk_hlBBvOEE-A0&code_challenge_method=S256&redirect_uri=http%3A%2F%2Flocalhost%3A8888%2Foauth%2Fdone&response_type=code&scope=psp.onlinepayment%3Awrite+psp.accountsettings%3Awrite+psp.webhook%3Awrite&state=abcdef", url)
	})

	t.Run("Get access token", func(t *testing.T) {
		verifier := func(t *testing.T, r *http.Request, req GetTokenRequest) {
			assert.Equal(t, "/v1/token", r.RequestURI)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			clientID, clientSecret, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "123", clientID)
			assert.Equal(t, "456", clientSecret)

			assert.Equal(t, "http://localhost:8080/oauth/done", req.RedirectURI)
			assert.Equal(t, "mycode", req.Code)
			assert.Equal(t, "randomString", req.CodeVerifier)
		}
		responder := func(t *testing.T, req GetTokenRequest) GetTokenResponse {
			return GetTokenResponse{
				TokenType:    "bearer",
				ExpiresIn:    12345,
				AccessToken:  "abc123",
				Scope:        adyenExampleScopes,
				RefreshToken: "rst456",
			}
		}

		ts, cleanup := RunGetAccessTokenServer(t, unserializeGetTokenRequest, verifier, responder, serializeGetTokenResponse)
		defer cleanup()

		providers := providers.NewProviders()
		providers.Set("adyen", "123", "456", ts.URL, ts.URL)

		client := NewOAuthClient(providers, nil)
		_, err := client.GetAccessToken(context.TODO(), GetTokenRequest{
			ProviderName: "adyen",
			RedirectURI:  "http://localhost:8080/oauth/done",
			Code:         "mycode",
			CodeVerifier: "randomString",
		})
		assert.NoError(t, err)
	})

	t.Run("Get access token: new way", func(t *testing.T) {
		mux := http.NewServeMux()
		ts := httptest.NewServer(mux)
		defer ts.Close()

		exampleResp := GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    12345,
			AccessToken:  "abc123",
			Scope:        adyenExampleScopes,
			RefreshToken: "rst456",
		}
		providers := providers.NewProviders()
		adyenProvider, err := providers.Get("adyen")
		assert.NoError(t, err)
		mux.HandleFunc(adyenProvider.TokenEndpoint.Path, func(w http.ResponseWriter, r *http.Request) {
			// request validation logic
			assert.Equal(t, "/v1/token", r.RequestURI)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			clientID, clientSecret, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "123", clientID)
			assert.Equal(t, "456", clientSecret)

			err := r.ParseForm()
			assert.NoError(t, err)

			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			assert.Equal(t, "http://localhost:8080/oauth/done", r.Form.Get("redirect_uri"))
			assert.Equal(t, "mycode", r.Form.Get("code"))
			assert.Equal(t, "exampleHash", r.Form.Get("code_verifier"))

			// write json response
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(exampleResp)
			assert.NoError(t, err)
		})

		providers.Set("adyen", "123", "456", ts.URL, ts.URL)

		client := NewOAuthClient(providers, nil)
		resp, err := client.GetAccessToken(context.TODO(), GetTokenRequest{
			ProviderName: "adyen",
			RedirectURI:  "http://localhost:8080/oauth/done",
			Code:         "mycode",
			CodeVerifier: "exampleHash",
		})
		assert.NoError(t, err)
		assert.Equal(t, exampleResp, resp)
	})

	t.Run("Refresh access token", func(t *testing.T) {
		mux := http.NewServeMux()
		ts := httptest.NewServer(mux)
		defer ts.Close()

		exampleResp := GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    12345,
			AccessToken:  "anewbc123",
			Scope:        adyenExampleScopes,
			RefreshToken: "newrst456",
		}
		providers := providers.NewProviders()
		adyenProvider, err := providers.Get("adyen")
		assert.NoError(t, err)

		mux.HandleFunc(adyenProvider.TokenEndpoint.Path, func(w http.ResponseWriter, r *http.Request) {
			// request validation logic
			assert.Equal(t, "/v1/token", r.RequestURI)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			clientID, clientSecret, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "123", clientID)
			assert.Equal(t, "456", clientSecret)

			err := r.ParseForm()
			assert.NoError(t, err)

			assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
			assert.Equal(t, "r999", r.Form.Get("refresh_token"))

			// write json response
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(exampleResp)
			assert.NoError(t, err)
		})

		providers.Set("adyen", "123", "456", ts.URL, ts.URL)

		client := NewOAuthClient(providers, nil)
		resp, err := client.RefreshAccessToken(context.TODO(), RefreshTokenRequest{
			ProviderName: "adyen",
			RefreshToken: "r999",
		})
		assert.NoError(t, err)
		assert.Equal(t, exampleResp, resp)
	})
}

func serializeGetTokenResponse(t *testing.T, resp GetTokenResponse, w http.ResponseWriter) {
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(resp)
	assert.NoError(t, err)
}

func unserializeGetTokenRequest(t *testing.T, r *http.Request) GetTokenRequest {
	err := r.ParseForm()
	assert.NoError(t, err)

	return GetTokenRequest{
		RedirectURI:  r.Form.Get("redirect_uri"),
		Code:         r.Form.Get("code"),
		CodeVerifier: r.Form.Get("code_verifier"),
	}
}
