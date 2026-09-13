// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

type linkPackageResult struct {
	Owner  string `json:"owner"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Repo   string `json:"repo"`
	Status string `json:"status"`
}

type unlinkPackageResult struct {
	Owner  string `json:"owner"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

var (
	LinkPackageTool = mcp.NewTool(
		LinkPackageToolName,
		mcp.WithDescription("Link a package name to a repository under the same owner. repo is the repository name, not owner/repo. Cross-owner is 404 upstream. Link is on the package name, not a version; every version of that name shares the repository. HTTP 201 returns {owner, type, name, repo, status: \"linked\"}; 4xx/5xx stay errors."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.PackageRepo)),
	)

	UnlinkPackageTool = mcp.NewTool(
		UnlinkPackageToolName,
		mcp.WithDescription("Unlink a package name from its repository. Unlink is on the package name, not a version. HTTP 204 returns {owner, type, name, status: \"unlinked\"}; 4xx/5xx stay errors."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
	)
)

func LinkPackageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called LinkPackageFn")
	args := req.GetArguments()

	owner, packageType, name, err := requiredPackageNameCoords(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, _ := args["repo"].(string)
	if repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	if strings.Contains(repo, "/") {
		return to.ErrorResult(fmt.Errorf("repo must be the repository name under the same owner, not owner/repo, got %q", repo))
	}

	path := forgejo.APIPath("packages", owner, packageType, name, "-", "link", repo)
	if err := forgejo.DoJSON(ctx, http.MethodPost, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("link package: %w", err))
	}
	return to.TextResult(linkPackageResult{
		Owner:  owner,
		Type:   packageType,
		Name:   name,
		Repo:   repo,
		Status: "linked",
	})
}

func UnlinkPackageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UnlinkPackageFn")
	args := req.GetArguments()

	owner, packageType, name, err := requiredPackageNameCoords(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := forgejo.APIPath("packages", owner, packageType, name, "-", "unlink")
	if err := forgejo.DoJSON(ctx, http.MethodPost, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("unlink package: %w", err))
	}
	return to.TextResult(unlinkPackageResult{
		Owner:  owner,
		Type:   packageType,
		Name:   name,
		Status: "unlinked",
	})
}
