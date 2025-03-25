package to

import (
	"encoding/json"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/log"
	"github.com/mark3labs/mcp-go/mcp"
)

type textResult struct {
	Result any
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

func ErrorResult(err error) (*mcp.CallToolResult, error) {
	log.Errorf(err.Error())
	return nil, err
}
