package myhttp

import (
	"context"
	"net/http"
)

type ResponseWriter interface {
	WriteError(c context.Context, w http.ResponseWriter, errorCode int, err error)
	Write(c context.Context, w http.ResponseWriter, httpStatus int, resp interface{})
}
