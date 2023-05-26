package challenge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

//go:generate mockgen -source=challenge.go -package challenge -destination random_stringer_mock.go RandomStringer
type RandomStringer interface {
	Create() (string, error)
}

type randomStringer struct {
}

func NewRandomStringer() RandomStringer {
	return &randomStringer{}
}

func (s randomStringer) Create() (string, error) {
	return randomBytesInHex(32)
}

func randomBytesInHex(count int) (string, error) {
	buf := make([]byte, count)

	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		return "", fmt.Errorf("could not generate random %d bytes: %v", count, err)
	}

	return hex.EncodeToString(buf), nil
}

func Create(value string) (string, string, error) {
	sha2 := sha256.New()

	_, err := io.WriteString(sha2, value)
	if err != nil {
		return "", "", fmt.Errorf("could not write challenge: %v", err)
	}

	codeChallenge := base64.RawURLEncoding.EncodeToString(sha2.Sum(nil))

	return "S256", codeChallenge, nil
}
