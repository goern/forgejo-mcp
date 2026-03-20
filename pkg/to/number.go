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
