package user

import (
	"context"
	"encoding/json"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetMyUserInfoToolName = "get_my_user_info"
)

var (
	GetMyUserInfoTool = mcp.NewTool(
		GetMyUserInfoToolName,
		mcp.WithDescription("Get my user info"),
	)
)

func GetUserInfoFn(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, _, err := gitea.Client().GetMyUserInfo()
	if err != nil {
		return mcp.NewToolResultError("Get My User Info Error"), err
	}

	result, err := json.Marshal(user)
	if err != nil {
		return mcp.NewToolResultError("marshal my user info error"), err
	}
	return mcp.NewToolResultText(string(result)), nil
}
