package repo

import (
	"context"
	"errors"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/ptr"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

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
		mcp.WithDescription("Create repo"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Repo name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("owner", mcp.Description("Owner/org name")),
		mcp.WithBoolean("private", mcp.Description(params.Private)),
		mcp.WithString("issue_labels", mcp.Description("Issue label set")),
		mcp.WithBoolean("auto_init", mcp.Description("Auto-initialize")),
		mcp.WithBoolean("template", mcp.Description("Template repo")),
		mcp.WithString("gitignores", mcp.Description("Gitignore templates")),
		mcp.WithString("license", mcp.Description("License")),
		mcp.WithString("readme", mcp.Description("README content")),
		mcp.WithString("default_branch", mcp.Description("Default branch")),
	)

	ForkRepoTool = mcp.NewTool(
		ForkRepoToolName,
		mcp.WithDescription("Fork repo"),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("organization", mcp.Description("Org name")),
		mcp.WithString("name", mcp.Description("Fork name")),
	)

	ListMyReposTool = mcp.NewTool(
		ListMyReposToolName,
		mcp.WithDescription("List my repos"),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
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
	name, ok := req.GetArguments()["name"].(string)
	if !ok {
		return to.ErrorResult(errors.New("repository name is required"))
	}
	description, _ := req.GetArguments()["description"].(string)
	owner, _ := req.GetArguments()["owner"].(string)
	private, _ := req.GetArguments()["private"].(bool)
	issueLabels, _ := req.GetArguments()["issue_labels"].(string)
	autoInit, _ := req.GetArguments()["auto_init"].(bool)
	template, _ := req.GetArguments()["template"].(bool)
	gitignores, _ := req.GetArguments()["gitignores"].(string)
	license, _ := req.GetArguments()["license"].(string)
	readme, _ := req.GetArguments()["readme"].(string)
	defaultBranch, _ := req.GetArguments()["default_branch"].(string)

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
	user, ok := req.GetArguments()["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("user name is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(errors.New("repository name is required"))
	}
	organization, ok := req.GetArguments()["organization"].(string)
	organizationPtr := ptr.To(organization)
	if !ok || organization == "" {
		organizationPtr = nil
	}
	name, ok := req.GetArguments()["name"].(string)
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
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 100
	}
	opt := forgejo_sdk.ListReposOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	repos, _, err := forgejo.Client().ListMyRepos(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list my repositories error: %v", err))
	}

	return to.TextResult(repos)
}
