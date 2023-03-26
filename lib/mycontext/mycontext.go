package mycontext

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func ContextFromHTTPRequest(r *http.Request) context.Context {
	var trace string

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	traceContext := r.Header.Get("X-	Cloud-Trace-Context")
	traceParts := strings.Split(traceContext, "/")
	if len(traceParts) > 0 && len(traceParts[0]) > 0 {
		trace = fmt.Sprintf("projects/%s/traces/%s", projectID, traceParts[0])
	}

	ctx := context.WithValue(context.Background(), "Cloud-Trace-Context", trace)

	return ctx
}
