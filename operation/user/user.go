package user

import (
	"context"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/to"

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

func GetUserInfoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, _, err := gitea.Client().GetMyUserInfo()
	if err != nil {
		return mcp.NewToolResultError("Get My User Info Error"), err
	}

	return to.TextResult(user)
}
