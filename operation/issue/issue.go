package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetIssueByIndexToolName = "get_issue_by_index"
)

var (
	GetIssueByIndexTool = mcp.NewTool(
		GetIssueByIndexToolName,
		mcp.WithDescription("get issue by index"),
		mcp.WithString(
			"owner",
			mcp.Required(),
			mcp.Description("repository owner"),
		),
		mcp.WithString(
			"repo",
			mcp.Required(),
			mcp.Description("repository name"),
		),
		mcp.WithNumber(
			"index",
			mcp.Required(),
			mcp.Description("repository issue index"),
		),
	)
)

func GetIssueByIndexFn(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	owner := request.Params.Arguments["owner"].(string)
	repo := request.Params.Arguments["repo"].(string)
	index := request.Params.Arguments["index"].(float64)
	issue, _, err := gitea.Client().GetIssue(owner, repo, int64(index))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get %v/%v/issue/%v err", owner, repo, int64(index))), err
	}

	result, err := json.Marshal(issue)
	if err != nil {
		return mcp.NewToolResultError("marshal issue err"), err
	}
	return mcp.NewToolResultText(string(result)), nil
}
