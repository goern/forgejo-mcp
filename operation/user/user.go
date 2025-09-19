package user

import (
	"context"
	"fmt"
	"time"

	"forgejo.org/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
	"forgejo.org/forgejo/forgejo-mcp/pkg/to"

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
	ctx, _ = log.WithMCPContext(ctx, GetMyUserInfoToolName)
	start := time.Now()

	log.LogMCPToolStart(ctx, GetMyUserInfoToolName, map[string]interface{}{})

	user, resp, err := forgejo.Client().GetMyUserInfo()
	duration := time.Since(start)

	// Log API call details
	forgejo.LogAPICall(ctx, "GET", "/user", duration, resp.StatusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, GetMyUserInfoToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("get user info err: %v", err))
	}

	log.LogMCPToolComplete(ctx, GetMyUserInfoToolName, duration, fmt.Sprintf("Retrieved info for user: %s", user.UserName))

	return to.TextResult(user)
}
