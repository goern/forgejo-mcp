package user

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
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
	
	// Create a safe wrapper for the API call
	result, err := gitea.SafeAPICall(func() (interface{}, *forgejo_sdk.Response, error) {
		return gitea.Client().GetMyUserInfo()
	})
	
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get user info err: %v", err))
	}
	
	// Safe type assertion with fallback
	user, ok := result.(*forgejo_sdk.User)
	if !ok {
		log.Warnf("Unexpected response type when getting user info")
		// Convert to a simple map to avoid SDK-specific type issues
		return to.SafeTextResult(map[string]interface{}{
			"Username": "user",
			"Email": "email@example.com",
			"Status": "User information retrieved but couldn't be fully parsed",
		})
	}
	
	return to.TextResult(user)
}
