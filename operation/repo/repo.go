package repo

import (
	"context"

	gitea_sdk "code.gitea.io/sdk/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateRepoToolName  = "create_repo"
	ListMyReposToolName = "list_my_repos"
)

var (
	CreateRepoTool = mcp.NewTool(
		CreateRepoToolName,
		mcp.WithDescription("Create repository"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the repository to create"), mcp.DefaultString("test")),
		mcp.WithString("description", mcp.Description("Description of the repository to create"), mcp.DefaultString("")),
		mcp.WithBoolean("private", mcp.Description("Whether the repository is private"), mcp.DefaultBool(true)),
		mcp.WithString("issue_labels", mcp.Description("Issue Label set to use"), mcp.DefaultString("")),
		mcp.WithBoolean("auto_init", mcp.Description("Whether the repository should be auto-intialized?"), mcp.DefaultBool(false)),
		mcp.WithBoolean("template", mcp.Description("Whether the repository is template"), mcp.DefaultBool(false)),
		mcp.WithString("gitignores", mcp.Description("Gitignores to use"), mcp.DefaultString("")),
		mcp.WithString("license", mcp.Description("License to use"), mcp.DefaultString("MIT")),
		mcp.WithString("readme", mcp.Description("Readme of the repository to create"), mcp.DefaultString("")),
		mcp.WithString("default_branch", mcp.Description("DefaultBranch of the repository (used when initializes and in template)"), mcp.DefaultString("main")),
	)

	ListMyReposTool = mcp.NewTool(
		ListMyReposToolName,
		mcp.WithDescription("List my repositories"),
		mcp.WithNumber("page", mcp.Required(), mcp.Description("Page number"), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("pageSize", mcp.Required(), mcp.Description("Page size number"), mcp.DefaultNumber(10), mcp.Min(1)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(CreateRepoTool, CreateRepoFn)
	s.AddTool(ListMyReposTool, ListMyReposFn)

	// Branch
	s.AddTool(CreateBranchTool, CreateBranchFn)
}

func CreateRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateRepoFn")
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

	opt := gitea_sdk.CreateRepoOption{
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
	repo, _, err := gitea.Client().CreateRepo(opt)
	if err != nil {
		return nil, err
	}
	return to.TextResult(repo)
}

func ListMyReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListMyReposFn")
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		return mcp.NewToolResultError("get page number error"), nil
	}
	size, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		return mcp.NewToolResultError("get page size number error"), nil
	}
	opt := gitea_sdk.ListReposOptions{
		ListOptions: gitea_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(size),
		},
	}
	repos, _, err := gitea.Client().ListMyRepos(opt)
	if err != nil {
		return mcp.NewToolResultError("List my repositories error"), err
	}

	return to.TextResult(repos)
}
