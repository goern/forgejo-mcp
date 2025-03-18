package to

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

func TextResult(v any) (*mcp.CallToolResult, error) {
	result, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError("marshal result error"), err
	}
	return mcp.NewToolResultText(string(result)), nil
}
