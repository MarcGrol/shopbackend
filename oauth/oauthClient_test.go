package oauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthClient(t *testing.T) {
	t.Run("Compose auth url", func(t *testing.T) {

		oauthClient := NewOAuthClient("123", "456")
		url, err := oauthClient.ComposeAuthURL(context.TODO(), ComposeAuthURLRequest{
			CompletionURL: "http://localhost:8888/oauth/done",
			Scope:         "scope 1 scope 2",
			State:         "abcdef",
			CodeVerifier:  "exampleHash",
		})
		assert.NoError(t, err)
		assert.Equal(t, "https://ca-test.adyen.com/ca/ca/oauth/connect.shtml?client_id=123&code_challenge=a4SPfcpynli7bwu--Wt2kOtp7WnyYfxoEOyM3r8TrFE&code_challenge_method=S256&redirect_uri=http%3A%2F%2Flocalhost%3A8888%2Foauth%2Fdone&response_type=code&scope=scope+1+scope+2&state=abcdef", url)
	})

	t.Run("Get access token", func(t *testing.T) {
		// TODO
	})

}
