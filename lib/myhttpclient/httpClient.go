package myhttpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

const (
	timeout = 5 * time.Second
)

type jsonHTTPClient struct {
}

func newJSONHTTPClient() HTTPSender {
	return &jsonHTTPClient{}
}

func (c jsonHTTPClient) Send(ctx context.Context, method string, url string, body []byte) (int, []byte, error) {
	httpReq, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, []byte{}, fmt.Errorf("error creating http request for %s %s: %s", method, url, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	reqDump, err := httputil.DumpRequestOut(httpReq, true)
	if err == nil {
		fmt.Printf("HTTP-req:\n%s", string(reqDump))
	}

	log.Printf("HTTP request: %s %s", method, url)

	httpClient := &http.Client{
		Timeout: timeout,
	}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return 0, []byte{}, fmt.Errorf("error sending %s %s: %s", method, url, err)
	}

	defer httpResp.Body.Close()

	respDump, err := httputil.DumpResponse(httpResp, true)
	if err != nil {
		fmt.Printf("HTTP-resp:\n%s", string(respDump))
	}

	respPayload, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return 0, []byte{}, fmt.Errorf("error reading response %s %s: %s", method, url, err)
	}

	log.Printf("HTTP resp: %d", httpResp.StatusCode)

	return httpResp.StatusCode, respPayload, nil
}
