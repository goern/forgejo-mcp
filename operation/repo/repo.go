package repo

import (
	"context"
	"errors"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"forgejo.org/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
	"forgejo.org/forgejo/forgejo-mcp/pkg/ptr"
	"forgejo.org/forgejo/forgejo-mcp/pkg/to"

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
		mcp.WithString("owner", mcp.Description("Owner/organization name for the repository")),
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
		return to.ErrorResult(errors.New("repository name is required"))
	}
	description, _ := req.Params.Arguments["description"].(string)
	owner, _ := req.Params.Arguments["owner"].(string)
	private, _ := req.Params.Arguments["private"].(bool)
	issueLabels, _ := req.Params.Arguments["issue_labels"].(string)
	autoInit, _ := req.Params.Arguments["auto_init"].(bool)
	template, _ := req.Params.Arguments["template"].(bool)
	gitignores, _ := req.Params.Arguments["gitignores"].(string)
	license, _ := req.Params.Arguments["license"].(string)
	readme, _ := req.Params.Arguments["readme"].(string)
	defaultBranch, _ := req.Params.Arguments["default_branch"].(string)

	opt := forgejo_sdk.CreateRepoOption{
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
	var repo *forgejo_sdk.Repository
	var err error
	if owner != "" {
		repo, _, err = forgejo.Client().CreateOrgRepo(owner, opt)
	} else {
		repo, _, err = forgejo.Client().CreateRepo(opt)
	}
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create repo err: %v", err))
	}
	return to.TextResult(repo)
}

func ForkRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ForkRepoFn")
	user, ok := req.Params.Arguments["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("user name is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(errors.New("repository name is required"))
	}
	organization, ok := req.Params.Arguments["organization"].(string)
	organizationPtr := ptr.To(organization)
	if !ok || organization == "" {
		organizationPtr = nil
	}
	name, ok := req.Params.Arguments["name"].(string)
	namePtr := ptr.To(name)
	if !ok || name == "" {
		namePtr = nil
	}
	opt := forgejo_sdk.CreateForkOption{
		Organization: organizationPtr,
		Name:         namePtr,
	}
	_, _, err := forgejo.Client().CreateFork(user, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("fork repository error %v", err))
	}
	return to.TextResult("Fork success")
}

func ListMyReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListMyReposFn")
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	pageSize, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		pageSize = 100
	}
	opt := forgejo_sdk.ListReposOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(pageSize),
		},
	}
	repos, _, err := forgejo.Client().ListMyRepos(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list my repositories error: %v", err))
	}

	return to.TextResult(repos)
}
