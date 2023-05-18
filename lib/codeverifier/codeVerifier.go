package codeverifier

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

type Verifier struct {
	Value string
}

func NewVerifierFrom(hexdata string) *Verifier {
	return &Verifier{
		Value: hexdata,
	}
}
func NewVerifier() (*Verifier, error) {
	value, err := randomBytesInHex(32)
	if err != nil {
		return nil, err
	}

	return &Verifier{
		Value: value,
	}, nil
}

func (v *Verifier) GetValue() string {
	return v.Value
}

func (v *Verifier) CreateChallenge() (string, string, error) {
	sha2 := sha256.New()

	_, err := io.WriteString(sha2, v.Value)
	if err != nil {
		return "", "", fmt.Errorf("could not write challenge: %v", err)
	}

	codeChallenge := base64.RawURLEncoding.EncodeToString(sha2.Sum(nil))

	return "S256", codeChallenge, nil
}

func randomBytesInHex(count int) (string, error) {
	buf := make([]byte, count)

	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		return "", fmt.Errorf("could not generate %d Value bytes: %v", count, err)
	}

	return hex.EncodeToString(buf), nil
}
