package repo

import (
	"context"

	"code.gitea.io/sdk/gitea"
	giteaPkg "gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/to"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	CreateRepoToolName  = "create_repo"
	ListMyReposToolName = "list_my_repos"
)

var (
	CreateRepoTool = mcp.NewTool(
		CreateRepoToolName,
		CreateRepoOpt...,
	)

	CreateRepoOpt = []mcp.ToolOption{
		mcp.WithDescription("Create repository"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the repository to create")),
		mcp.WithString("description", mcp.Description("Description of the repository to create")),
		mcp.WithBoolean("private", mcp.Description("Whether the repository is private")),
		mcp.WithString("issue_labels", mcp.Description("Issue Label set to use")),
		mcp.WithBoolean("auto_init", mcp.Description("Whether the repository should be auto-intialized?")),
		mcp.WithBoolean("template", mcp.Description("Whether the repository is template")),
		mcp.WithString("gitignores", mcp.Description("Gitignores to use")),
		mcp.WithString("license", mcp.Description("License to use")),
		mcp.WithString("readme", mcp.Description("Readme of the repository to create")),
		mcp.WithString("default_branch", mcp.Description("DefaultBranch of the repository (used when initializes and in template)")),
	}

	ListMyReposTool = mcp.NewTool(
		ListMyReposToolName,
		ListMyReposOpt...,
	)

	ListMyReposOpt = []mcp.ToolOption{
		mcp.WithDescription("List my repositories"),
		mcp.WithNumber(
			"page",
			mcp.Description("Page number"),
			mcp.DefaultNumber(1),
			mcp.Min(1),
		),
		mcp.WithNumber(
			"pageSize",
			mcp.Description("Page size number"),
			mcp.DefaultNumber(10),
			mcp.Min(1),
		),
	}
)

func CreateRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.Params.Arguments["name"].(string)
	description := req.Params.Arguments["description"].(string)
	private := req.Params.Arguments["private"].(bool)
	issueLabels := req.Params.Arguments["issue_labels"].(string)
	autoInit := req.Params.Arguments["auto_init"].(bool)
	template := req.Params.Arguments["template"].(bool)
	gitignores := req.Params.Arguments["gitignores"].(string)
	license := req.Params.Arguments["license"].(string)
	readme := req.Params.Arguments["readme"].(string)
	defaultBranch := req.Params.Arguments["default_branch"].(string)

	opt := gitea.CreateRepoOption{
		Name:          name,
		Description:   description,
		Private:       private,
		IssueLabels:   issueLabels,
		AutoInit:      autoInit,
		Template:      template,
		Gitignores:    gitignores,
		License:       license,
		Readme:        readme,
		DefaultBranch: defaultBranch,
	}
	repo, _, err := giteaPkg.Client().CreateRepo(opt)
	if err != nil {
		return nil, err
	}
	return to.TextResult(repo)
}

func ListMyReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		return mcp.NewToolResultError("get page number error"), nil
	}
	size, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		return mcp.NewToolResultError("get page size number error"), nil
	}
	opts := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{
			Page:     int(page),
			PageSize: int(size),
		},
	}
	repos, _, err := giteaPkg.Client().ListMyRepos(opts)
	if err != nil {
		return mcp.NewToolResultError("List my repositories error"), err
	}

	return to.TextResult(repos)
}
