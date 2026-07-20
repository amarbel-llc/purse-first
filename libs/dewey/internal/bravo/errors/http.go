package errors

import (
	"fmt"

	hs "code.linenisgreat.com/purse-first/libs/dewey/internal/0/http_statuses"
)

// HTTPStatusCarrier is the semantic tag attached to errors that carry an
// HTTP status code. Detected via errors.As through wrapped chains.
type HTTPStatusCarrier interface {
	error
	HTTPStatusCode() hs.Code
}

// HTTPRenderer is the explicit transformer used at HTTP wire/UX boundaries
// to convert a domain error into its on-the-wire HTTP form. Domain code
// returns errors satisfying HTTPStatusCarrier; HTTP handlers/middleware
// call AsHTTPError(err) to obtain the rendered form.
type HTTPRenderer interface {
	HTTPStatusCarrier
	HTTPRender() error
}

// HTTPStatusError carries an HTTP status code as semantics, not identity.
// Error() returns the wrapped message, leaving the HTTP rendering to
// HTTPRender() / AsHTTPError() at the wire boundary. This means CLI tools,
// log lines, and other non-HTTP consumers get useful output without
// needing to walk past hidden-wrapper layers.
type HTTPStatusError struct {
	status     hs.Code
	underlying error
}

// MakeHTTPStatusError constructs an HTTPStatusError directly. Prefer the
// status-specific constructors (BadRequestf, Conflictf, …) where
// possible.
func MakeHTTPStatusError(code hs.Code, underlying error) HTTPStatusError {
	return HTTPStatusError{status: code, underlying: underlying}
}

func (e HTTPStatusError) Error() string {
	if e.underlying == nil {
		return e.status.String()
	}
	return e.underlying.Error()
}

func (e HTTPStatusError) Unwrap() error {
	return e.underlying
}

func (e HTTPStatusError) HTTPStatusCode() hs.Code {
	return e.status
}

// HTTPRender returns the wire-shaped error: Error() renders
// "HTTP: <status>" with the underlying still reachable via Unwrap.
func (e HTTPStatusError) HTTPRender() error {
	return httpRendering{status: e.status, underlying: e.underlying}
}

// Errorf builds a new HTTPStatusError sharing this one's status,
// wrapping a newly-formatted error. Preserves the receiver-pattern
// constructor (Err422UnprocessableEntity.Errorf(...)) used by existing
// callers and tests.
func (e HTTPStatusError) Errorf(format string, args ...any) HTTPStatusError {
	return HTTPStatusError{
		status:     e.status,
		underlying: fmt.Errorf(format, args...),
	}
}

// Wrap builds a new HTTPStatusError sharing this one's status, wrapping
// the given underlying error.
func (e HTTPStatusError) Wrap(underlying error) HTTPStatusError {
	return HTTPStatusError{status: e.status, underlying: underlying}
}

// WithStack attaches a stack frame around this error.
func (e HTTPStatusError) WithStack() error {
	return WrapSkip(1, e)
}

// httpRendering is the wire-shaped HTTP error, produced only by
// HTTPRender() at the wire/UX boundary.
type httpRendering struct {
	status     hs.Code
	underlying error
}

func (h httpRendering) Error() string {
	return fmt.Sprintf("HTTP: %s", h.status.String())
}

func (h httpRendering) Unwrap() error {
	return h.underlying
}

func (h httpRendering) HTTPStatusCode() hs.Code {
	return h.status
}

func (h httpRendering) HTTPRender() error {
	return h
}

// statusString is the bare-text Error() implementation for sentinels.
type statusString hs.Code

func (s statusString) Error() string {
	return hs.Code(s).String()
}

// Status sentinels. Each is a valid HTTPStatusError whose Error()
// renders the status string ("499 Client Closed Request" etc.).
// Useful as both bare cancellation tokens (ctx.Cancel(Err499...)) and
// as constructor receivers (Err400BadRequest.Errorf("invalid %s", x)).
var (
	Err400BadRequest = HTTPStatusError{
		status:     hs.Code400BadRequest,
		underlying: statusString(hs.Code400BadRequest),
	}
	Err405MethodNotAllowed = HTTPStatusError{
		status:     hs.Code405MethodNotAllowed,
		underlying: statusString(hs.Code405MethodNotAllowed),
	}
	Err409Conflict = HTTPStatusError{
		status:     hs.Code409Conflict,
		underlying: statusString(hs.Code409Conflict),
	}
	Err422UnprocessableEntity = HTTPStatusError{
		status:     hs.Code422UnprocessableEntity,
		underlying: statusString(hs.Code422UnprocessableEntity),
	}
	Err499ClientClosedRequest = HTTPStatusError{
		status:     hs.Code499ClientClosedRequest,
		underlying: statusString(hs.Code499ClientClosedRequest),
	}
	Err500InternalServerError = HTTPStatusError{
		status:     hs.Code500InternalServerError,
		underlying: statusString(hs.Code500InternalServerError),
	}
	Err501NotImplemented = HTTPStatusError{
		status:     hs.Code501NotImplemented,
		underlying: statusString(hs.Code501NotImplemented),
	}
)

// AsHTTPError walks err's chain for an HTTPRenderer and returns the
// wire-shaped error. HTTP handlers call this to convert a domain error
// into the response shape. Returns (nil, false) if no HTTP-tagged
// error is in the chain.
func AsHTTPError(err error) (error, bool) {
	var r HTTPRenderer
	if !As(err, &r) {
		return nil, false
	}
	return r.HTTPRender(), true
}
