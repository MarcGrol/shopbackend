package myerrors

import (
	"fmt"
	"testing"
)

func TestErrors(t *testing.T) {
	myErr := fmt.Errorf("my error")

	testCases := []struct {
		msg        string
		in         error
		httpStatus int
	}{
		{
			msg:        "No http error",
			in:         myErr,
			httpStatus: 500,
		},
		{
			msg:        "Invalid input error",
			in:         NewInvalidInputError(myErr),
			httpStatus: 400,
		},
		{
			msg:        "Not found error",
			in:         NewNotFoundError(myErr),
			httpStatus: 404,
		},
		{
			msg:        "Authentication error",
			in:         NewAuthenticationError(myErr),
			httpStatus: 403,
		},
		{
			msg:        "Internal error",
			in:         NewInternalError(myErr),
			httpStatus: 500,
		},
		{
			msg:        "Not implemented error",
			in:         NewNotImplementedError(myErr),
			httpStatus: 501,
		},
		{
			msg:        "Not available error",
			in:         NewUnavailableError(myErr),
			httpStatus: 503,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.msg, func(t *testing.T) {
			httpStatus := GetHTTPStatus(tc.in)
			if httpStatus != tc.httpStatus {
				t.Errorf("HttpStatus: got %v, want %v", httpStatus, tc.httpStatus)
			}
		})
	}
}
