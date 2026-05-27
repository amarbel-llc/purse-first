package errors

import (
	"fmt"

	hs "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/http_statuses"
)

// BadRequest tags an existing error with HTTP 400. No-op if the chain
// already carries 400.
func BadRequest(err error) HTTPStatusError {
	var existing HTTPStatusError
	if As(err, &existing) && existing.HTTPStatusCode() == hs.Code400BadRequest {
		return existing
	}
	return HTTPStatusError{status: hs.Code400BadRequest, underlying: err}
}

// BadRequestf formats and tags with HTTP 400.
func BadRequestf(format string, args ...any) HTTPStatusError {
	return HTTPStatusError{
		status:     hs.Code400BadRequest,
		underlying: fmt.Errorf(format, args...),
	}
}

// BadRequestWrapf is an alias for BadRequestf retained for backwards
// compatibility; both behave identically (pre-#107 they were already
// the same).
func BadRequestWrapf(format string, args ...any) HTTPStatusError {
	return BadRequestf(format, args...)
}

// Conflictf formats and tags with HTTP 409.
func Conflictf(format string, args ...any) HTTPStatusError {
	return HTTPStatusError{
		status:     hs.Code409Conflict,
		underlying: fmt.Errorf(format, args...),
	}
}

// UnprocessableEntityf formats and tags with HTTP 422.
func UnprocessableEntityf(format string, args ...any) HTTPStatusError {
	return HTTPStatusError{
		status:     hs.Code422UnprocessableEntity,
		underlying: fmt.Errorf(format, args...),
	}
}

// NotImplementedf formats and tags with HTTP 501.
func NotImplementedf(format string, args ...any) HTTPStatusError {
	return HTTPStatusError{
		status:     hs.Code501NotImplemented,
		underlying: fmt.Errorf(format, args...),
	}
}

// Is400BadRequest reports whether err's chain carries HTTP 400.
func Is400BadRequest(err error) bool {
	return IsHTTPError(err, hs.Code400BadRequest)
}

// Is499ClientClosedRequest reports whether err's chain carries HTTP 499.
func Is499ClientClosedRequest(err error) bool {
	return IsHTTPError(err, hs.Code499ClientClosedRequest)
}

// IsHTTPError reports whether err's chain carries the given status code.
// Uses errors.As against the HTTPStatusCarrier interface so it works
// across both HTTPStatusError (domain) and httpRendering (wire).
func IsHTTPError(target error, statusCode hs.Code) bool {
	var carrier HTTPStatusCarrier
	if !As(target, &carrier) {
		return false
	}
	return carrier.HTTPStatusCode() == statusCode
}
