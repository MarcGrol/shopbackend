package myhttp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/MarcGrol/shopbackend/myerrors"
)

type errorResponse struct {
	ErrorCode int
	Message   string
}

func WriteError(w http.ResponseWriter, errorCode int, err error) {
	Write(w, myerrors.GetHttpStatus(err), errorResponse{
		ErrorCode: errorCode,
		Message:   err.Error(),
	})
}

func Write(w http.ResponseWriter, httpStatus int, resp interface{}) {
	w.WriteHeader(httpStatus)
	encoder := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	encoder.SetIndent("", "\t")
	err := encoder.Encode(resp)
	if err != nil {
		log.Printf("Error writing error response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}


func HostnameWithScheme(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

