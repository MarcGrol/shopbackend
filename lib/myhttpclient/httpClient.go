package myhttpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const httpClientTimeout = 20 * time.Second

type realClient struct {
}

func newRealClient() HTTPSender {
	return &realClient{}
}

func (_ realClient) Send(c context.Context, method string, url string, body []byte) (int, []byte, error) {
	httpReq, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, []byte{}, fmt.Errorf("Error creating http request for %s %s: %s", method, url, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	log.Printf("HTTP request: %s %s", method, url)
	httpClient := &http.Client{
		Timeout: httpClientTimeout,
	}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return 0, []byte{}, fmt.Errorf("Error sending %s %s: %s", method, url, err)
	}
	defer httpResp.Body.Close()

	respPayload, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return 0, []byte{}, fmt.Errorf("Error reading response %s %s: %s", method, url, err)
	}
	log.Printf("HTTP resp: %d", httpResp.StatusCode)

	return httpResp.StatusCode, respPayload, nil
}
