package to

import (
	"fmt"
	"strconv"
)

// Float64 extracts a float64 from an interface{} value.
// Handles both JSON number (float64) and string-encoded numbers,
// which can occur when MCP clients serialize number parameters as strings.
func Float64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as number: %w", val, err)
		}
		return f, nil
	case nil:
		return 0, fmt.Errorf("parameter is nil")
	default:
		return 0, fmt.Errorf("unexpected type %T for number parameter", v)
	}
}

// Float64Ok is like Float64 but returns (0, false) for nil or non-numeric
// values instead of an error. Use it when callers need to distinguish
// "parameter absent" from "parameter malformed" — e.g. a tool that accepts
// exactly one of two alternative numeric inputs.
// An empty string is treated as absent.
func Float64Ok(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		if val == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
