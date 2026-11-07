package resource

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

// ResourceError wraps a forgejo URI and an HTTP status code into a structured error
// for MCP resource handlers.
type ResourceError struct {
	URI     string
	Code    int
	Message string
}

func (e *ResourceError) Error() string {
	return fmt.Sprintf("resource %q: %s (code %d)", e.URI, e.Message, e.Code)
}

// statusError attaches the HTTP status of an SDK response to the SDK's error,
// so MapForgejoError can classify by status instead of by message text.
type statusError struct {
	code int
	err  error
}

func (e *statusError) Error() string { return fmt.Sprintf("%d %s", e.code, e.err.Error()) }
func (e *statusError) Unwrap() error { return e.err }

// WithStatus records the HTTP status code of a Forgejo SDK response on err.
// Use it where an SDK call returned a non-nil *Response alongside err:
//
//	return nil, resource.MapForgejoError(uri, resource.WithStatus(resp.StatusCode, err))
func WithStatus(code int, err error) error {
	if err == nil {
		return nil
	}
	return &statusError{code: code, err: err}
}

// httpStatus returns the HTTP status carried by err, if any: a *statusError
// from WithStatus, or a *forgejo.HTTPError from the raw-HTTP helpers.
func httpStatus(err error) (int, bool) {
	var se *statusError
	if errors.As(err, &se) {
		return se.code, true
	}
	var he *forgejo.HTTPError
	if errors.As(err, &he) {
		return he.StatusCode, true
	}
	return 0, false
}

// MapForgejoError maps a Forgejo error to a ResourceError. Returns a
// *ResourceError with:
//   - Code -32602 for invalid-params errors (ErrInvalidParams sentinel)
//   - Code -32002 for HTTP 403 access-denied responses
//   - Code -32003 for HTTP 404 not-found responses
//   - Code -32603 (internal error) for all other errors
//
// The HTTP status is read from the error's type (see WithStatus and
// forgejo.HTTPError), never from a bare number in its message: error text
// carries the request URL, and a port, owner, or page name containing "403"
// must not turn a 404 into access denied. Only an error with no typed status
// falls back to the message: a leading "403 " / "404 " (the legacy
// fmt.Errorf("%d %s", resp.StatusCode, err) form) or the full status phrase
// "403 Forbidden" / "404 Not Found".
func MapForgejoError(uri string, err error) *ResourceError {
	if err == nil {
		return nil
	}
	// Check sentinel first — covers all parse-time invalid-input errors.
	if errors.Is(err, ErrInvalidParams) {
		return &ResourceError{URI: uri, Code: -32602, Message: "invalid params: " + err.Error()}
	}
	msg := err.Error()
	code, ok := httpStatus(err)
	if !ok {
		switch {
		case errors.Is(err, forgejo.ErrNotFound), strings.HasPrefix(msg, "404 "), strings.Contains(msg, "404 Not Found"):
			code = http.StatusNotFound
		case strings.HasPrefix(msg, "403 "), strings.Contains(msg, "403 Forbidden"):
			code = http.StatusForbidden
		}
	}
	switch code {
	case http.StatusForbidden:
		return &ResourceError{URI: uri, Code: -32002, Message: "access denied: " + msg}
	case http.StatusNotFound:
		return &ResourceError{URI: uri, Code: -32003, Message: "not found: " + msg}
	default:
		return &ResourceError{URI: uri, Code: -32603, Message: "internal error: " + msg}
	}
}
