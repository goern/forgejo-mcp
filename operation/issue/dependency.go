// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

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
		mcp.WithDescription("List issues that the given issue depends on. Pagination uses page (1-based) and limit (page size); the response echoes page and limit so callers can fetch the next page. Returns an empty list if the issue has no dependencies. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	ListIssueDependentsTool = mcp.NewTool(
		ListIssueDependentsToolName,
		mcp.WithDescription("List issues that depend on the given issue. Pagination uses page (1-based) and limit (page size); the response echoes page and limit so callers can fetch the next page. Returns an empty list if no issue depends on it. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	AddIssueDependencyTool = mcp.NewTool(
		AddIssueDependencyToolName,
		mcp.WithDescription("Make one issue depend on another issue. The issue identified by index will depend on depends_on_index. The dependency may live in a different repository (cross-repo): pass depends_on_owner/depends_on_repo, which default to owner/repo. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("depends_on_index", mcp.Required(), mcp.Description("Issue index that the given issue should depend on")),
		mcp.WithString("depends_on_owner", mcp.Description("Owner of the repository the dependency issue lives in (defaults to owner)")),
		mcp.WithString("depends_on_repo", mcp.Description("Repository the dependency issue lives in (defaults to repo)")),
	)

	RemoveIssueDependencyTool = mcp.NewTool(
		RemoveIssueDependencyToolName,
		mcp.WithDescription("Remove a dependency from the given issue. The dependency on dependency_index is removed from the issue identified by index. For a cross-repo dependency, pass dependency_owner/dependency_repo, which default to owner/repo. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("dependency_index", mcp.Required(), mcp.Description("Issue index to remove as a dependency")),
		mcp.WithString("dependency_owner", mcp.Description("Owner of the repository the dependency issue lives in (defaults to owner)")),
		mcp.WithString("dependency_repo", mcp.Description("Repository the dependency issue lives in (defaults to repo)")),
	)
)

// issueMetaBody is the Forgejo/Gitea IssueMeta request body used by the
// dependency and blocks mutation endpoints. It requires owner, repo, and index
// rather than a single dependency_issue_index field.
type issueMetaBody struct {
	Index int64  `json:"index"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// paginatedDependencyResult wraps a list of dependency issues with the page
// metadata needed for resumability. The shape echoes the page/limit parameters
// so callers can fetch the next page.
type paginatedDependencyResult struct {
	Page   int                  `json:"page"`
	Limit  int                  `json:"limit"`
	Issues []*forgejo_sdk.Issue `json:"issues"`
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
	page, limit := parsePageLimit(req.GetArguments())

	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "dependencies") + fmt.Sprintf("?page=%d&limit=%d", page, limit)
	issues := []*forgejo_sdk.Issue{}
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &issues); err != nil {
		return to.ErrorResult(fmt.Errorf("list issue dependencies err: %w", err))
	}
	return to.TextResult(paginatedDependencyResult{Page: page, Limit: limit, Issues: issues})
}

func ListIssueDependentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueDependentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	page, limit := parsePageLimit(req.GetArguments())

	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "blocks") + fmt.Sprintf("?page=%d&limit=%d", page, limit)
	issues := []*forgejo_sdk.Issue{}
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &issues); err != nil {
		return to.ErrorResult(fmt.Errorf("list issue dependents err: %w", err))
	}
	return to.TextResult(paginatedDependencyResult{Page: page, Limit: limit, Issues: issues})
}

// crossRepoArgs resolves the optional owner/repo pair naming the repository a
// dependency issue lives in, defaulting to the target issue's own repository.
//
// Absent and malformed are kept apart, the way pkg/to separates Float64 from
// Float64Ok: an omitted, nil or empty argument means "the target's own repo",
// but a value of the wrong type is an error. Collapsing the two would write a
// SAME-repo dependency for a caller that asked for a different one and report
// success, which is worse than refusing.
func crossRepoArgs(args map[string]any, ownerKey, repoKey, defaultOwner, defaultRepo string) (string, string, error) {
	depOwner, err := optionalString(args, ownerKey, defaultOwner)
	if err != nil {
		return "", "", err
	}
	depRepo, err := optionalString(args, repoKey, defaultRepo)
	if err != nil {
		return "", "", err
	}
	return depOwner, depRepo, nil
}

// optionalString returns the string under key, fallback when it is absent, nil
// or empty, and an error naming the argument when it is present but not a
// string.
func optionalString(args map[string]any, key, fallback string) (string, error) {
	raw, present := args[key]
	if !present || raw == nil {
		return fallback, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected a string, got %T", key, raw)
	}
	if s == "" {
		return fallback, nil
	}
	return s, nil
}

func parsePageLimit(args map[string]any) (page, limit int) {
	pageFloat, _ := to.Float64(args["page"])
	page = int(pageFloat)
	if page == 0 {
		page = 1
	}
	limitFloat, _ := to.Float64(args["limit"])
	limit = int(limitFloat)
	if limit == 0 {
		limit = 20
	}
	return page, limit
}

func AddIssueDependencyFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddIssueDependencyFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	dependsOn, err := to.Float64(req.GetArguments()["depends_on_index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("depends_on_index: %w", err))
	}
	depOwner, depRepo, err := crossRepoArgs(req.GetArguments(), "depends_on_owner", "depends_on_repo", owner, repo)
	if err != nil {
		return to.ErrorResult(err)
	}

	// Scoped to same-repo (including the defaulted case, since depOwner/depRepo
	// already fall back to owner/repo above): a same-index dependency in a
	// DIFFERENT repository is a legitimate cross-repo dependency, not a
	// self-dependency (see TestAddIssueDependency_CrossRepoSameIndexAllowed).
	// index and depends_on_index are validated above so a missing/malformed
	// required argument surfaces as its own clear error instead of both
	// silently coercing to 0 and being misreported as "cannot depend on itself".
	// EqualFold on owner and repo: Forgejo treats both case-insensitively, so an
	// uppercased spelling of the same repository is the same repository. Forgejo
	// rejects the self-dependency server-side either way; comparing case-blind is
	// about failing early with the clearer message instead of a generic API error.
	if strings.EqualFold(depOwner, owner) && strings.EqualFold(depRepo, repo) &&
		int64(index) == int64(dependsOn) {
		return to.ErrorResult(fmt.Errorf("an issue cannot depend on itself"))
	}

	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "dependencies")
	body := issueMetaBody{Index: int64(dependsOn), Owner: depOwner, Repo: depRepo}
	if err := forgejo.DoJSON(ctx, http.MethodPost, path, body, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("add issue dependency err: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Issue #%d now depends on %s/%s#%d", int64(index), depOwner, depRepo, int64(dependsOn)))
}

func RemoveIssueDependencyFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveIssueDependencyFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	dependencyIndex, err := to.Float64(req.GetArguments()["dependency_index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("dependency_index: %w", err))
	}
	depOwner, depRepo, err := crossRepoArgs(req.GetArguments(), "dependency_owner", "dependency_repo", owner, repo)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := forgejo.APIPath("repos", owner, repo, "issues", int64(index), "dependencies")
	body := issueMetaBody{Index: int64(dependencyIndex), Owner: depOwner, Repo: depRepo}
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, body, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("remove issue dependency err: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Removed dependency on %s/%s#%d from issue #%d", depOwner, depRepo, int64(dependencyIndex), int64(index)))
}
