package myerrors

import (
	"fmt"
	"log"
	"net/http"
)

type httpErrorCoder interface {
	error
	GetHTTPErrorCode() int
}

type httpError struct {
	httpCode int
	err      error
}

func (e httpError) Error() string {
	return fmt.Sprintf("status: %d, err: %s", e.httpCode, e.err.Error())
}

func (e httpError) GetHTTPErrorCode() int {
	return e.httpCode
}

func newError(httpCode int, err error) *httpError {
	return &httpError{
		httpCode: httpCode,
		err:      err,
	}
}

func NewInvalidInputError(err error) *httpError {
	log.Printf("Returning 400: %s", err.Error())
	return newError(http.StatusBadRequest, err)
}

func NewUnsupportedMediaTypeError(err error) *httpError {
	return newError(http.StatusUnsupportedMediaType, err)
}

func NewInvalidInputErrorf(format string, args ...interface{}) *httpError {
	return NewInvalidInputError(fmt.Errorf(format, args...))
}

func NewNotFoundError(err error) *httpError {
	return newError(http.StatusNotFound, err)
}

func NewAuthenticationError(err error) *httpError {
	return newError(http.StatusForbidden, err)
}

func NewInternalError(err error) *httpError {
	return newError(http.StatusInternalServerError, err)
}

func NewNotImplementedError(err error) *httpError {
	return newError(http.StatusNotImplemented, err)
}

func NewUnavailableError(err error) *httpError {
	return newError(http.StatusServiceUnavailable, err)
}

func GetHttpStatus(err error) int {
	if err != nil {
		myError, ok := err.(httpErrorCoder)
		if ok {
			return myError.GetHTTPErrorCode()
		}
	}
	return http.StatusInternalServerError
}
