package repo

import (
	"context"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoContentsToolName = "list_repo_contents"
	GetRepoTreeToolName      = "get_repo_tree"
)

var (
	ListRepoContentsTool = mcp.NewTool(
		ListRepoContentsToolName,
		mcp.WithDescription("List the files and directories at a given path in a repository. Use path=\"\" to list the repository root. Returns one level of entries at the specified path; for a full recursive file tree use get_repo_tree with recursive=true."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory path within the repository (empty string lists the repository root)")),
	)

	GetRepoTreeTool = mcp.NewTool(
		GetRepoTreeToolName,
		mcp.WithDescription("Get the Git tree of a repository. With recursive=true, returns the complete file tree in a single response (subject to the server's tree-endpoint size cap); use this when you need all paths at once. With recursive=false (default), returns only the top-level entries of the tree."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithBoolean("recursive", mcp.Description("Return the complete file tree in one response (subject to server cap); default false returns top-level entries only")),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(1000), mcp.Min(1)),
	)
)

func ListRepoContentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoContentsFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	ref, _ := req.GetArguments()["ref"].(string)
	path, _ := req.GetArguments()["path"].(string)

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	contents, _, err := client.ListContents(owner, repo, ref, path)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo contents err: %v", err))
	}
	return to.TextResult(contents)
}

func GetRepoTreeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetRepoTreeFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	ref, ok := req.GetArguments()["ref"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("ref is required"))
	}
	recursive, _ := req.GetArguments()["recursive"].(bool)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 1000
	}

	opt := forgejo_sdk.GetTreesOptions{
		Recursive: recursive,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	tree, _, err := client.GetTrees(owner, repo, ref, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get repo tree err: %v", err))
	}
	return to.TextResult(tree)
}
