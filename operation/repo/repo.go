package repo

import (
	"context"
	"errors"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/ptr"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	gitea_sdk "code.gitea.io/sdk/gitea"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateRepoToolName  = "create_repo"
	ForkRepoToolName    = "fork_repo"
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

	ForkRepoTool = mcp.NewTool(
		ForkRepoToolName,
		mcp.WithDescription("Fork repository"),
		mcp.WithString("user", mcp.Required(), mcp.Description("User name of the repository to fork")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository name to fork")),
		mcp.WithString("organization", mcp.Description("Organization name to fork")),
		mcp.WithString("name", mcp.Description("Name of the forked repository")),
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
	s.AddTool(ForkRepoTool, ForkRepoFn)
	s.AddTool(ListMyReposTool, ListMyReposFn)

	// File
	s.AddTool(GetFileTool, GetFileFn)
	s.AddTool(CreateFileTool, CreateFileFn)
	s.AddTool(UpdateFileTool, UpdateFileFn)
	s.AddTool(DeleteFileTool, DeleteFileFn)

	// Branch
	s.AddTool(CreateBranchTool, CreateBranchFn)
	s.AddTool(DeleteBranchTool, DeleteBranchFn)
	s.AddTool(ListBranchesTool, ListBranchesFn)

	// Commit
	s.AddTool(ListRepoCommitsTool, ListRepoCommitsFn)
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

func ForkRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ForkRepoFn")
	user := req.Params.Arguments["user"].(string)
	repo := req.Params.Arguments["repo"].(string)
	opt := gitea_sdk.CreateForkOption{
		Organization: ptr.To(req.Params.Arguments["organization"].(string)),
		Name:         ptr.To(req.Params.Arguments["name"].(string)),
	}
	_, _, err := gitea.Client().CreateFork(user, repo, opt)
	if err != nil {
		return nil, fmt.Errorf("fork repository error %v", err)
	}
	return to.TextResult("Fork success")
}

func ListMyReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListMyReposFn")
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		return nil, errors.New("get page number error")
	}
	size, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		return nil, errors.New("get page size number error")
	}
	opt := gitea_sdk.ListReposOptions{
		ListOptions: gitea_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(size),
		},
	}
	repos, _, err := gitea.Client().ListMyRepos(opt)
	if err != nil {
		return nil, fmt.Errorf("list my repositories error %v", err)
	}

	return to.TextResult(repos)
}
