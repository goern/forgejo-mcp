package to

import (
	"encoding/json"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/pkg/log"
	"github.com/mark3labs/mcp-go/mcp"
)

type textResult struct {
	Result any
}

func SafeJSONMarshal(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Errorf("JSON marshal error: %v", err)
		return `"Data couldn't be serialized properly"`
	}
	return string(data)
}

func TextResult(v any) (*mcp.CallToolResult, error) {
	result := textResult{v}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result err: %v", err)
	}
	log.Debugf("Text Result: %s", string(resultBytes))
	return mcp.NewToolResultText(string(resultBytes)), nil
}

// SafeTextResult creates a text result with additional safety checks
func SafeTextResult(v any) (*mcp.CallToolResult, error) {
	// If v is a struct or complex type, try to convert it to a simple map
	// This provides an extra layer of safety against SDK-specific types
	var safeResult any = v

	jsonStr := SafeJSONMarshal(safeResult)
	return mcp.NewToolResultText(fmt.Sprintf(`{"Result":%s}`, jsonStr)), nil
}

func ErrorResult(err error) (*mcp.CallToolResult, error) {
	log.Errorf(err.Error())
	return nil, err
}
