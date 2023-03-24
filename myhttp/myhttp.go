package myhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/MarcGrol/shopbackend/myerrors"
	"github.com/MarcGrol/shopbackend/mylog"
)

type errorResponse struct {
	ErrorCode int
	Message   string
}

func NewWriter(logger mylog.Logger) ResponseWriter {
	return &responseWriter{
		logger: logger,
	}
}

type responseWriter struct {
	logger mylog.Logger
}

type EmptyResponse struct{}

func (rw responseWriter) WriteError(c context.Context, w http.ResponseWriter, errorCode int, err error) {
	httpStatus := myerrors.GetHttpStatus(err)
	rw.logger.Log(c, "", mylog.SeverityWarn, "Error response: http-status:%d, error-code:%d, error-msg:%s", httpStatus, errorCode, err)
	rw.write(w, httpStatus, errorResponse{
		ErrorCode: errorCode,
		Message:   err.Error(),
	})
}

func (rw responseWriter) Write(c context.Context, w http.ResponseWriter, httpStatus int, resp interface{}) {
	rw.logger.Log(c, "", mylog.SeverityInfo, "Success response: http-status:%d", httpStatus)
	rw.write(w, httpStatus, resp)
}

func (rw responseWriter) write(w http.ResponseWriter, httpStatus int, resp interface{}) {
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
