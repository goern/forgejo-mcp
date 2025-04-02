package user

import (
	"context"
	"fmt"

	"forgejo.com/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.com/forgejo/forgejo-mcp/pkg/log"
	"forgejo.com/forgejo/forgejo-mcp/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetMyUserInfoTool, GetUserInfoFn)
}

func GetUserInfoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetUserInfoFn")
	user, _, err := forgejo.Client().GetMyUserInfo()
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get user info err: %v", err))
	}

	return to.TextResult(user)
}
