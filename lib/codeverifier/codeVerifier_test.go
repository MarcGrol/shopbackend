package codeverifier

import (
	"testing"
)

func TestVerifier(t *testing.T) {
	tests := []struct {
		name         string
		verifierData string
		challenge    string
	}{
		{
			name:         "test example",
			verifierData: "05796efe18af079dc654bb88c68f5cd8b8a5d378e7cec8e9856258f95d3b0b5a",
			challenge:    "A-Y4cHhqIJi48r-V_cKdDRzlMJmC8zk_hlBBvOEE-A0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewVerifierFrom(tt.verifierData)
			method, challenge := v.CreateChallenge()
			if method != "S256" {
				t.Errorf("CreateChallenge() got = %v, want %v", method, "S256")
			}
			if challenge != tt.challenge {
				t.Errorf("CreateChallenge() got = %v, want %v", challenge, tt.challenge)
			}
		})
	}
}
