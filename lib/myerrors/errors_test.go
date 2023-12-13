package myerrors

import (
	"fmt"
	"testing"
)

func TestErrors(t *testing.T) {
	myErr := fmt.Errorf("my error")

	testCases := []struct {
		name       string
		in         error
		httpStatus int
		errorText  string
	}{
		{
			name:       "No http error",
			in:         myErr,
			httpStatus: 500,
			errorText:  "my error",
		},
		{
			name:       "Invalid input error",
			in:         NewInvalidInputError(myErr),
			httpStatus: 400,
			errorText:  "status: 400, err: my error",
		},
		{
			name:       "Invalid input errorf",
			in:         NewInvalidInputErrorf("%s: %d", myErr.Error(), 123),
			httpStatus: 400,
			errorText:  "status: 400, err: my error: 123",
		},
		{
			name:       "Authentication error",
			in:         NewAuthenticationError(myErr),
			httpStatus: 403,
			errorText:  "status: 403, err: my error",
		},
		{
			name:       "Not found error",
			in:         NewNotFoundError(myErr),
			httpStatus: 404,
			errorText:  "status: 404, err: my error",
		},
		{
			name:       "UnsupportedMedia error",
			in:         NewUnsupportedMediaTypeError(myErr),
			httpStatus: 415,
			errorText:  "status: 415, err: my error",
		},
		{
			name:       "Internal error",
			in:         NewInternalError(myErr),
			httpStatus: 500,
			errorText:  "status: 500, err: my error",
		},
		{
			name:       "Not implemented error",
			in:         NewNotImplementedError(myErr),
			httpStatus: 501,
			errorText:  "status: 501, err: my error",
		},
		{
			name:       "Not available error",
			in:         NewUnavailableError(myErr),
			httpStatus: 503,
			errorText:  "status: 503, err: my error",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpStatus := GetHTTPStatus(tc.in)
			if httpStatus != tc.httpStatus {
				t.Errorf("HttpStatus: got %v, want %v", httpStatus, tc.httpStatus)
			}
			if tc.errorText != tc.in.Error() {
				t.Errorf("%s: ErrorText: got %v, want %v", tc.name, tc.in.Error(), tc.errorText)

			}
		})
	}
}
