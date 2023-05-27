package oauthclient

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
	httpClientTimeout = 5 * time.Second
	debug             = false
)

type httpOAuthClient struct {
	username string
	password string
}

func newHTTPClient(username string, password string) *httpOAuthClient {
	return &httpOAuthClient{
		username: username,
		password: password,
	}
}

func (c httpOAuthClient) Send(ctx context.Context, method string, url string, body []byte) (int, []byte, error) {
	httpReq, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, []byte{}, fmt.Errorf("error creating http request for %s %s: %s", method, url, err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.username, c.password)

	if debug {
		reqDump, err := httputil.DumpRequestOut(httpReq, true)
		if err == nil {
			fmt.Printf("HTTP-req:\n%s", string(reqDump))
		}
	}

	httpClient := &http.Client{
		Timeout: httpClientTimeout,
	}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return 0, []byte{}, fmt.Errorf("error calling %s %s: %s", method, url, err)
	}
	defer httpResp.Body.Close()

	log.Printf("HTTP call to oauth: %s %s -> %d", method, url, httpResp.StatusCode)

	if debug {
		respDump, err := httputil.DumpResponse(httpResp, true)
		if err == nil {
			fmt.Printf("HTTP-resp:\n%s", string(respDump))
		}
	}

	respPayload, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return 0, []byte{}, fmt.Errorf("error reading response %s %s: %s", method, url, err)
	}

	return httpResp.StatusCode, respPayload, nil
}
