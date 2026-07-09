// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"net/http"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListIssueDependenciesToolName = "list_issue_dependencies"
	ListIssueDependentsToolName   = "list_issue_dependents"
	AddIssueDependencyToolName    = "add_issue_dependency"
	RemoveIssueDependencyToolName = "remove_issue_dependency"
)

var (
	ListIssueDependenciesTool = mcp.NewTool(
		ListIssueDependenciesToolName,
		mcp.WithDescription("List issues that the given issue depends on. Returns an empty list if the issue has no dependencies. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
	)

	ListIssueDependentsTool = mcp.NewTool(
		ListIssueDependentsToolName,
		mcp.WithDescription("List issues that depend on the given issue. Returns an empty list if no issue depends on it. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
	)

	AddIssueDependencyTool = mcp.NewTool(
		AddIssueDependencyToolName,
		mcp.WithDescription("Make one issue depend on another issue. The issue identified by index will depend on depends_on_index. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("depends_on_index", mcp.Required(), mcp.Description("Issue index that the given issue should depend on")),
	)

	RemoveIssueDependencyTool = mcp.NewTool(
		RemoveIssueDependencyToolName,
		mcp.WithDescription("Remove a dependency from the given issue. The dependency on dependency_index is removed from the issue identified by index. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("dependency_index", mcp.Required(), mcp.Description("Issue index to remove as a dependency")),
	)
)

// issueMetaBody is the request body used by the forge.he-int.de API for the
// dependency and blocks mutation endpoints. It requires owner, repo, and index
// rather than a single dependency_issue_index field.
type issueMetaBody struct {
	Index int64  `json:"index"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func RegisterDependencyTool(s *server.MCPServer) {
	s.AddTool(ListIssueDependenciesTool, ListIssueDependenciesFn)
	s.AddTool(ListIssueDependentsTool, ListIssueDependentsFn)
	s.AddTool(AddIssueDependencyTool, AddIssueDependencyFn)
	s.AddTool(RemoveIssueDependencyTool, RemoveIssueDependencyFn)
}

func ListIssueDependenciesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueDependenciesFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])

	path := fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies", owner, repo, int64(index))
	issues := []*forgejo_sdk.Issue{}
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &issues); err != nil {
		return to.ErrorResult(fmt.Errorf("list issue dependencies err: %w", err))
	}
	return to.TextResult(issues)
}

func ListIssueDependentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueDependentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])

	path := fmt.Sprintf("/repos/%s/%s/issues/%d/blocks", owner, repo, int64(index))
	issues := []*forgejo_sdk.Issue{}
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &issues); err != nil {
		return to.ErrorResult(fmt.Errorf("list issue dependents err: %w", err))
	}
	return to.TextResult(issues)
}

func AddIssueDependencyFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddIssueDependencyFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	dependsOn, _ := to.Float64(req.GetArguments()["depends_on_index"])

	if int64(index) == int64(dependsOn) {
		return to.ErrorResult(fmt.Errorf("an issue cannot depend on itself"))
	}

	path := fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies", owner, repo, int64(index))
	body := issueMetaBody{Index: int64(dependsOn), Owner: owner, Repo: repo}
	if err := forgejo.DoJSON(ctx, http.MethodPost, path, body, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("add issue dependency err: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Issue #%d now depends on issue #%d", int64(index), int64(dependsOn)))
}

func RemoveIssueDependencyFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveIssueDependencyFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	dependencyIndex, _ := to.Float64(req.GetArguments()["dependency_index"])

	path := fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies", owner, repo, int64(index))
	body := issueMetaBody{Index: int64(dependencyIndex), Owner: owner, Repo: repo}
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, body, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("remove issue dependency err: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Removed dependency on issue #%d from issue #%d", int64(dependencyIndex), int64(index)))
}
