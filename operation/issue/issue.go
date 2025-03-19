package issue

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetIssueByIndexToolName = "get_issue_by_index"
)

var (
	GetIssueByIndexTool = mcp.NewTool(
		GetIssueByIndexToolName,
		GetIssueByIndexOpt...,
	)

	GetIssueByIndexOpt = []mcp.ToolOption{
		mcp.WithDescription("get issue by index"),
		mcp.WithString(
			"owner",
			mcp.Required(),
			mcp.Description("repository owner"),
			mcp.DefaultString(""),
		),
		mcp.WithString(
			"repo",
			mcp.Required(),
			mcp.Description("repository name"),
			mcp.DefaultString(""),
		),
		mcp.WithNumber(
			"index",
			mcp.Required(),
			mcp.Description("repository issue index"),
			mcp.DefaultNumber(0),
		),
	}
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetIssueByIndexTool, GetIssueByIndexFn)
}

func GetIssueByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	index := req.Params.Arguments["index"].(float64)
	issue, _, err := gitea.Client().GetIssue(owner, repo, int64(index))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get %v/%v/issue/%v err", owner, repo, int64(index))), err
	}

	return to.TextResult(issue)
}
