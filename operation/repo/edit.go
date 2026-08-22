// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/ptr"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
	GetRepoTool = mcp.NewTool(
		GetRepoToolName,
		mcp.WithDescription("Get a single repository by owner and name."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)

	EditRepoTool = mcp.NewTool(
		EditRepoToolName,
		mcp.WithDescription("Edit repository settings (PATCH — only supplied fields change). Providing no fields is an error. name renames the repository; subsequent calls must use the new name. Does not change topics (those use a separate API)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Description("New repository name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("website", mcp.Description(params.Website)),
		mcp.WithString("default_branch", mcp.Description("Default branch")),
		mcp.WithBoolean("private", mcp.Description(params.Private)),
		mcp.WithBoolean("template", mcp.Description("Template repo")),
		mcp.WithBoolean("archived", mcp.Description("Archive the repository")),
		mcp.WithBoolean("has_issues", mcp.Description("Enable the issues unit")),
		mcp.WithBoolean("has_wiki", mcp.Description("Enable the wiki unit")),
		mcp.WithBoolean("has_pull_requests", mcp.Description("Enable pull requests")),
		mcp.WithBoolean("has_projects", mcp.Description("Enable the projects unit")),
		mcp.WithBoolean("has_releases", mcp.Description("Enable releases")),
		mcp.WithBoolean("has_packages", mcp.Description("Enable packages")),
		mcp.WithBoolean("has_actions", mcp.Description("Enable Actions")),
	)
)

func loadRepo(ctx context.Context, owner, repo string) (*forgejo_sdk.Repository, error) {
	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, err
	}
	r, _, err := client.GetRepo(owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get repo: %w", err)
	}
	return r, nil
}

func GetRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetRepoFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}

	r, err := loadRepo(ctx, owner, repo)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(r)
}

func EditRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditRepoFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}

	opt := forgejo_sdk.EditRepoOption{}
	hasField := false

	if v, ok := args["name"].(string); ok && v != "" {
		opt.Name = ptr.To(v)
		hasField = true
	}
	if v, ok := args["description"].(string); ok {
		opt.Description = ptr.To(v)
		hasField = true
	}
	if v, ok := args["website"].(string); ok {
		opt.Website = ptr.To(v)
		hasField = true
	}
	if v, ok := args["default_branch"].(string); ok && v != "" {
		opt.DefaultBranch = ptr.To(v)
		hasField = true
	}
	if v, ok := args["private"].(bool); ok {
		opt.Private = ptr.To(v)
		hasField = true
	}
	if v, ok := args["template"].(bool); ok {
		opt.Template = ptr.To(v)
		hasField = true
	}
	if v, ok := args["archived"].(bool); ok {
		opt.Archived = ptr.To(v)
		hasField = true
	}
	if v, ok := args["has_issues"].(bool); ok {
		opt.HasIssues = ptr.To(v)
		hasField = true
	}
	if v, ok := args["has_wiki"].(bool); ok {
		opt.HasWiki = ptr.To(v)
		hasField = true
	}
	if v, ok := args["has_pull_requests"].(bool); ok {
		opt.HasPullRequests = ptr.To(v)
		hasField = true
	}
	if v, ok := args["has_projects"].(bool); ok {
		opt.HasProjects = ptr.To(v)
		hasField = true
	}
	if v, ok := args["has_releases"].(bool); ok {
		opt.HasReleases = ptr.To(v)
		hasField = true
	}
	if v, ok := args["has_packages"].(bool); ok {
		opt.HasPackages = ptr.To(v)
		hasField = true
	}
	if v, ok := args["has_actions"].(bool); ok {
		opt.HasActions = ptr.To(v)
		hasField = true
	}

	if !hasField {
		return to.ErrorResult(fmt.Errorf("edit_repo: at least one setting field must be provided"))
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	r, _, err := client.EditRepo(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit repo: %w", err))
	}
	return to.TextResult(r)
}
