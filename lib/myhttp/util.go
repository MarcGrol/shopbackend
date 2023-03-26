package myhttp

import (
	"fmt"
	"net/http"
)

func HostnameWithScheme(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}
