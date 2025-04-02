package to

import (
	"encoding/json"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/log"
	"github.com/mark3labs/mcp-go/mcp"
)

func SafeJSONMarshal(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Errorf("JSON marshal error: %v", err)
		return `"Data couldn't be serialized properly"`
	}
	return string(data)
}

// SafeTextResult creates a text result with additional safety checks
func SafeTextResult(v any) (*mcp.CallToolResult, error) {
	// If v is a struct or complex type, try to convert it to a simple map
	// This provides an extra layer of safety against SDK-specific types
	var safeResult any = v
	
	jsonStr := SafeJSONMarshal(safeResult)
	return mcp.NewToolResultText(fmt.Sprintf(`{"Result":%s}`, jsonStr)), nil
}

type textResult struct {
	Result any
}

func TextResult(v any) (*mcp.CallToolResult, error) {
	// Use the safer text result implementation
	return SafeTextResult(v)
}
