package resource

import (
	"errors"
	"fmt"
	"strings"
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

// MapForgejoError maps a forgejo SDK error to a ResourceError using the error
// message to detect HTTP status codes. Returns a *ResourceError with:
//   - Code -32002 for HTTP 403 access-denied responses
//   - Code -32003 for HTTP 404 not-found responses
//   - Code -32602 for invalid-params errors (ErrInvalidParams sentinel or parser phrases)
//   - Code -32603 (internal error) for all other errors
func MapForgejoError(uri string, err error) *ResourceError {
	if err == nil {
		return nil
	}
	// Check sentinel first — covers all parse-time invalid-input errors.
	if errors.Is(err, ErrInvalidParams) {
		return &ResourceError{URI: uri, Code: -32602, Message: "invalid params: " + err.Error()}
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "403") || strings.Contains(msg, "Forbidden") || strings.Contains(msg, "forbidden"):
		return &ResourceError{URI: uri, Code: -32002, Message: "access denied: " + msg}
	case strings.Contains(msg, "404") || strings.Contains(msg, "Not Found") || strings.Contains(msg, "not found"):
		return &ResourceError{URI: uri, Code: -32003, Message: "not found: " + msg}
	default:
		return &ResourceError{URI: uri, Code: -32603, Message: "internal error: " + msg}
	}
}
