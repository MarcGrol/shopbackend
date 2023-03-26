package myhttpclient

import (
	"context"
)

type HTTPSender interface {
	Send(c context.Context, method string, url string, body []byte) (int, []byte, error)
}

func New() HTTPSender {
	return newRealClient()
}
