package myhttp

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/mylog"
)

type ResponseWriter interface {
	WriteError(c context.Context, w http.ResponseWriter, errorCode int, err error)
	Write(c context.Context, w http.ResponseWriter, httpStatus int, resp interface{})
}

type errorResponse struct {
	ErrorCode int
	Message   string
}

type SuccessResponse struct {
	Message string
}

func NewWriter(logger mylog.Logger) ResponseWriter {
	return &responseWriter{
		logger: logger,
	}
}

type responseWriter struct {
	logger mylog.Logger
}

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
