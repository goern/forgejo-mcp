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
		mcp.WithNumber("pageSize", mcp.Required(), mcp.Description("Page size number"), mcp.DefaultNumber(100), mcp.Min(1)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(CreateRepoTool, CreateRepoFn)
	s.AddTool(ForkRepoTool, ForkRepoFn)
	s.AddTool(ListMyReposTool, ListMyReposFn)

	// File
	s.AddTool(GetFileContentTool, GetFileContentFn)
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
	name, ok := req.Params.Arguments["name"].(string)
	if !ok {
		return nil, errors.New("repository name is required")
	}
	description, _ := req.Params.Arguments["description"].(string)
	private, _ := req.Params.Arguments["private"].(bool)
	issueLabels, _ := req.Params.Arguments["issue_labels"].(string)
	autoInit, _ := req.Params.Arguments["auto_init"].(bool)
	template, _ := req.Params.Arguments["template"].(bool)
	gitignores, _ := req.Params.Arguments["gitignores"].(string)
	license, _ := req.Params.Arguments["license"].(string)
	readme, _ := req.Params.Arguments["readme"].(string)
	defaultBranch, _ := req.Params.Arguments["default_branch"].(string)

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
	user, ok := req.Params.Arguments["user"].(string)
	if !ok {
		return nil, errors.New("user name is required")
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return nil, errors.New("repository name is required")
	}
	organization, _ := req.Params.Arguments["organization"].(string)
	name, _ := req.Params.Arguments["name"].(string)
	opt := gitea_sdk.CreateForkOption{
		Organization: ptr.To(organization),
		Name:         ptr.To(name),
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
