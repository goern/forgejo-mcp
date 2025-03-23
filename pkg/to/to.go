package to

import (
	"encoding/json"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/log"
	"github.com/mark3labs/mcp-go/mcp"
)

func TextResult(v any) (*mcp.CallToolResult, error) {
	result, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result err: %v", err)
	}
	log.Debugf("Text Result: %s", string(result))
	return mcp.NewToolResultText(string(result)), nil
}
