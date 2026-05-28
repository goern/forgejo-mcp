package resource

import (
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
//   - Code -32602 for invalid-params errors (e.g. unknown URI kind from the parser)
//   - Code -32603 (internal error) for all other errors
func MapForgejoError(uri string, err error) *ResourceError {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "403") || strings.Contains(msg, "Forbidden") || strings.Contains(msg, "forbidden"):
		return &ResourceError{URI: uri, Code: -32002, Message: "access denied: " + msg}
	case strings.Contains(msg, "404") || strings.Contains(msg, "Not Found") || strings.Contains(msg, "not found"):
		return &ResourceError{URI: uri, Code: -32003, Message: "not found: " + msg}
	// Parser signals invalid URI parameters (e.g. unknown comment kind, non-numeric index) with
	// phrases like "kind must be" or "index must be numeric". Map these to -32602 invalid params.
	case strings.Contains(msg, "kind must be") || strings.Contains(msg, "index must be numeric") || strings.Contains(msg, "invalid params"):
		return &ResourceError{URI: uri, Code: -32602, Message: "invalid params: " + msg}
	default:
		return &ResourceError{URI: uri, Code: -32603, Message: "internal error: " + msg}
	}
}
