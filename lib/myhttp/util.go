package myhttp

import (
	"fmt"
	"net/http"
	"os"
)

func GuessHostnameWithScheme() string {
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "marcsexperiment" {
		return "https://www.marcgrolconsultancy.nl"
	}

	return "http://localhost:8080"
}

func HostnameWithScheme(r *http.Request) string {
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "marcsexperiment" {
		return "https://www.marcgrolconsultancy.nl"
	}

	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, r.Host)
}
