package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthClient(t *testing.T) {
	t.Run("Compose auth url", func(t *testing.T) {

		oauthClient := NewOAuthClient("123", "456", "https://ca-test.adyen.com")
		url, err := oauthClient.ComposeAuthURL(context.TODO(), ComposeAuthURLRequest{
			CompletionURL: "http://localhost:8888/oauth/done",
			Scope:         exampleScope,
			State:         "abcdef",
			CodeVerifier:  "exampleHash",
		})
		assert.NoError(t, err)
		assert.Equal(t, "https://ca-test.adyen.com/ca/ca/oauth/connect.shtml?client_id=123&code_challenge=a4SPfcpynli7bwu--Wt2kOtp7WnyYfxoEOyM3r8TrFE&code_challenge_method=S256&redirect_uri=http%3A%2F%2Flocalhost%3A8888%2Foauth%2Fdone&response_type=code&scope=psp.onlinepayment%3Awrite+psp.accountsettings%3Awrite+psp.webhook%3Awrite&state=abcdef", url)
	})

	t.Run("Get access token", func(t *testing.T) {
		mux := http.NewServeMux()
		ts := httptest.NewServer(mux)
		defer ts.Close()

		exampleResp := GetTokenResponse{
			TokenType:    "bearer",
			ExpiresIn:    12345,
			AccessToken:  "abc123",
			Scope:        exampleScope,
			RefreshToken: "rst456",
		}
		mux.HandleFunc(tokenURL, func(w http.ResponseWriter, r *http.Request) {
			// request validation logic
			assert.Equal(t, "/v1/token", r.RequestURI)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			username, password, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "123", username)
			assert.Equal(t, "456", password)

			err := r.ParseForm()
			assert.NoError(t, err)

			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			assert.Equal(t, "123", r.Form.Get("client_id"))
			assert.Equal(t, "http://localhost:8080/oauth/done", r.Form.Get("redirect_uri"))
			assert.Equal(t, "mycode", r.Form.Get("code"))
			assert.Equal(t, "exampleHash", r.Form.Get("code_verifier"))

			// write json response
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(exampleResp)
			assert.NoError(t, err)
		})

		client := NewOAuthClient("123", "456", ts.URL)
		resp, err := client.GetAccessToken(context.TODO(), GetTokenRequest{
			RedirectUri:  "http://localhost:8080/oauth/done",
			Code:         "mycode",
			CodeVerifier: "exampleHash",
		})
		assert.NoError(t, err)
		assert.Equal(t, exampleResp, resp)
	})
}
