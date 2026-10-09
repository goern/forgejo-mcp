// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/ptr"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateRepoLabelToolName = "create_repo_label"
	EditRepoLabelToolName   = "edit_repo_label"
	DeleteRepoLabelToolName = "delete_repo_label"
	GetRepoLabelToolName    = "get_repo_label"

	CreateOrgLabelToolName = "create_org_label"
	EditOrgLabelToolName   = "edit_org_label"
	DeleteOrgLabelToolName = "delete_org_label"
	GetOrgLabelToolName    = "get_org_label"
)

var (
	CreateRepoLabelTool = mcp.NewTool(
		CreateRepoLabelToolName,
		mcp.WithDescription("Create a repository label. Returns the created label including its numeric id, exclusive, and is_archived. exclusive=true requires a scoped name (a '/' not at either end)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("Label description")),
		mcp.WithBoolean("exclusive", mcp.Description(params.LabelExclusive)),
		mcp.WithBoolean("is_archived", mcp.Description(params.LabelArchived)),
	)

	EditRepoLabelTool = mcp.NewTool(
		EditRepoLabelToolName,
		mcp.WithDescription("Edit a repository label (PATCH — only supplied fields change). Providing no fields is an error. Optional exclusive and is_archived; exclusive=true with a new name requires a scoped name (a '/' not at either end). Exclusive-only edit does not re-read the current name."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("New label description")),
		mcp.WithBoolean("exclusive", mcp.Description(params.LabelExclusive)),
		mcp.WithBoolean("is_archived", mcp.Description(params.LabelArchived)),
	)

	DeleteRepoLabelTool = mcp.NewTool(
		DeleteRepoLabelToolName,
		mcp.WithDescription("Delete a repository label. By default refuses if the label is in use; set delete_mode=force to override."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("delete_mode", mcp.Description("safe (default): refuse if in use and report count. force: delete unconditionally.")),
	)

	GetRepoLabelTool = mcp.NewTool(
		GetRepoLabelToolName,
		mcp.WithDescription("Get a single repository label by ID. Includes exclusive and is_archived (false is present, not omitted)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
	)

	CreateOrgLabelTool = mcp.NewTool(
		CreateOrgLabelToolName,
		mcp.WithDescription("Create an organization-level label. Returns the created label including its numeric id, exclusive, and is_archived. exclusive=true requires a scoped name (a '/' not at either end)."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("Label description")),
		mcp.WithBoolean("exclusive", mcp.Description(params.LabelExclusive)),
		mcp.WithBoolean("is_archived", mcp.Description(params.LabelArchived)),
	)

	EditOrgLabelTool = mcp.NewTool(
		EditOrgLabelToolName,
		mcp.WithDescription("Edit an organization-level label (PATCH — only supplied fields change). Providing no fields is an error. Optional exclusive and is_archived; exclusive=true with a new name requires a scoped name (a '/' not at either end). Exclusive-only edit does not re-read the current name."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("New label description")),
		mcp.WithBoolean("exclusive", mcp.Description(params.LabelExclusive)),
		mcp.WithBoolean("is_archived", mcp.Description(params.LabelArchived)),
	)

	DeleteOrgLabelTool = mcp.NewTool(
		DeleteOrgLabelToolName,
		mcp.WithDescription("Delete an organization-level label. By default refuses if the label is in use; set delete_mode=force to override. Note: in-use count is best-effort over repos visible to the token and may under-count."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("delete_mode", mcp.Description("safe (default): refuse if in use and report count. force: delete unconditionally.")),
	)

	GetOrgLabelTool = mcp.NewTool(
		GetOrgLabelToolName,
		mcp.WithDescription("Get a single organization-level label by ID. Includes exclusive and is_archived (false is present, not omitted)."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
	)
)

func RegisterLabelTool(s *server.MCPServer) {
	s.AddTool(CreateRepoLabelTool, CreateRepoLabelFn)
	s.AddTool(EditRepoLabelTool, EditRepoLabelFn)
	s.AddTool(DeleteRepoLabelTool, DeleteRepoLabelFn)
	s.AddTool(GetRepoLabelTool, GetRepoLabelFn)
	s.AddTool(CreateOrgLabelTool, CreateOrgLabelFn)
	s.AddTool(EditOrgLabelTool, EditOrgLabelFn)
	s.AddTool(DeleteOrgLabelTool, DeleteOrgLabelFn)
	s.AddTool(GetOrgLabelTool, GetOrgLabelFn)
}

// labelDTO is the Forgejo label JSON we accept and return. exclusive and
// is_archived are always present on read so a false value is visible.
type labelDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url,omitempty"`
	Exclusive   bool   `json:"exclusive"`
	IsArchived  bool   `json:"is_archived"`
}

type labelCreateOption struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	Exclusive   *bool  `json:"exclusive,omitempty"`
	IsArchived  *bool  `json:"is_archived,omitempty"`
}

type labelPatchOption struct {
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
	Exclusive   *bool   `json:"exclusive,omitempty"`
	IsArchived  *bool   `json:"is_archived,omitempty"`
}

// normalizeColor accepts rrggbb or #rrggbb (6-digit only), lowercases,
// prepends # if absent. Returns an error for invalid input.
// 3-digit shorthand is intentionally rejected (upstream behavior unverified).
func normalizeColor(color string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(color))
	c = strings.TrimPrefix(c, "#")
	if len(c) != 6 {
		return "", fmt.Errorf("color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got %q", color)
	}
	for _, ch := range c {
		isHex := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
		if !isHex {
			return "", fmt.Errorf("color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got %q", color)
		}
	}
	return "#" + c, nil
}

// isScopedLabelName reports Forgejo's scoped-label rule: the name contains
// '/' not at either end.
func isScopedLabelName(name string) bool {
	return strings.Contains(name, "/") && !strings.HasPrefix(name, "/") && !strings.HasSuffix(name, "/")
}

func requireExclusiveScopedName(name string) error {
	if isScopedLabelName(name) {
		return nil
	}
	return fmt.Errorf("exclusive=true requires a scoped label name (a '/' not at either end), e.g. kind/bug, got %q", name)
}

func labelCreateFromArgs(args map[string]any) (labelCreateOption, error) {
	name, _ := args["name"].(string)
	colorRaw, _ := args["color"].(string)
	description, _ := args["description"].(string)
	color, err := normalizeColor(colorRaw)
	if err != nil {
		return labelCreateOption{}, err
	}
	opt := labelCreateOption{Name: name, Color: color}
	if description != "" {
		opt.Description = description
	}
	if exclusive, ok := args["exclusive"].(bool); ok {
		if exclusive {
			if err := requireExclusiveScopedName(name); err != nil {
				return labelCreateOption{}, err
			}
		}
		opt.Exclusive = ptr.To(exclusive)
	}
	if archived, ok := args["is_archived"].(bool); ok {
		opt.IsArchived = ptr.To(archived)
	}
	return opt, nil
}

func labelPatchFromArgs(args map[string]any, tool string) (labelPatchOption, error) {
	nameRaw, nameSet := args["name"].(string)
	colorRaw, colorSet := args["color"].(string)
	descRaw, descSet := args["description"].(string)
	exclusive, exclusiveSet := args["exclusive"].(bool)
	archived, archivedSet := args["is_archived"].(bool)

	if !nameSet && !colorSet && !descSet && !exclusiveSet && !archivedSet {
		return labelPatchOption{}, fmt.Errorf("%s: at least one of name, color, description, exclusive, is_archived must be provided", tool)
	}

	opt := labelPatchOption{}
	if nameSet && nameRaw != "" {
		opt.Name = ptr.To(nameRaw)
	}
	if colorSet && colorRaw != "" {
		color, err := normalizeColor(colorRaw)
		if err != nil {
			return labelPatchOption{}, err
		}
		opt.Color = ptr.To(color)
	}
	if descSet {
		opt.Description = ptr.To(descRaw)
	}
	if exclusiveSet {
		if exclusive && nameSet && nameRaw != "" {
			if err := requireExclusiveScopedName(nameRaw); err != nil {
				return labelPatchOption{}, err
			}
		}
		opt.Exclusive = ptr.To(exclusive)
	}
	if archivedSet {
		opt.IsArchived = ptr.To(archived)
	}
	return opt, nil
}

// repoLabelInUseCount returns the number of issues/PRs in the repo that carry
// the label (identified by name). Uses X-Total-Count from the SDK *Response.
func repoLabelInUseCount(ctx context.Context, client *forgejo_sdk.Client, owner, repo, labelName string) (int, error) {
	_, resp, err := client.ListRepoIssues(owner, repo, forgejo_sdk.ListIssueOption{
		State:       forgejo_sdk.StateType("all"),
		Labels:      []string{labelName},
		ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 1},
	})
	if err != nil {
		return 0, fmt.Errorf("count label usage: %w", err)
	}
	if resp != nil {
		if s := resp.Header.Get("X-Total-Count"); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				return n, nil
			}
		}
	}
	return 0, nil
}

// orgLabelInUseCount returns the best-effort count of issues/PRs in the org's
// visible repos that carry the label (identified by name). May under-count for
// repos the token cannot read.
func orgLabelInUseCount(ctx context.Context, client *forgejo_sdk.Client, org, labelName string) (int, error) {
	total := 0
	page := 1
	for {
		repos, _, err := client.ListOrgRepos(org, forgejo_sdk.ListOrgReposOptions{
			ListOptions: forgejo_sdk.ListOptions{Page: page, PageSize: 50},
		})
		if err != nil {
			return total, fmt.Errorf("enumerate org repos for label usage count: %w", err)
		}
		if len(repos) == 0 {
			break
		}
		for _, r := range repos {
			n, _ := repoLabelInUseCount(ctx, client, org, r.Name, labelName)
			total += n
		}
		page++
	}
	return total, nil
}

// ---- Repo label handlers ----

func CreateRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateRepoLabelFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)

	opt, err := labelCreateFromArgs(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	var label labelDTO
	if err := forgejo.DoJSON(ctx, http.MethodPost, forgejo.APIPath("repos", owner, repo, "labels"), opt, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("create repo label: %w", err))
	}
	return to.TextResult(label)
}

func EditRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditRepoLabelFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	idF, _ := to.Float64(args["id"])

	opt, err := labelPatchFromArgs(args, EditRepoLabelToolName)
	if err != nil {
		return to.ErrorResult(err)
	}

	var label labelDTO
	if err := forgejo.DoJSON(ctx, http.MethodPatch, forgejo.APIPath("repos", owner, repo, "labels", int64(idF)), opt, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("edit repo label: %w", err))
	}
	return to.TextResult(label)
}

func DeleteRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])
	deleteMode, _ := req.GetArguments()["delete_mode"].(string)
	id := int64(idF)

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	if deleteMode != "force" {
		label, _, err := client.GetRepoLabel(owner, repo, id)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("get repo label: %w", err))
		}
		count, err := repoLabelInUseCount(ctx, client, owner, repo, label.Name)
		if err != nil {
			return to.ErrorResult(err)
		}
		if count > 0 {
			return to.ErrorResult(fmt.Errorf("label %q is used by %d issue(s)/PR(s); set delete_mode=force to delete anyway", label.Name, count))
		}
	}

	resp, err := client.DeleteLabel(owner, repo, id)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return to.ErrorResult(fmt.Errorf("label %d not found", id))
		}
		return to.ErrorResult(fmt.Errorf("delete repo label: %w", err))
	}
	return to.TextResult(map[string]any{"deleted": true, "id": id})
}

func GetRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	var label labelDTO
	if err := forgejo.DoJSON(ctx, http.MethodGet, forgejo.APIPath("repos", owner, repo, "labels", int64(idF)), nil, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("get repo label: %w", err))
	}
	return to.TextResult(label)
}

// ---- Org label handlers ----

func CreateOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrgLabelFn")
	args := req.GetArguments()
	org, _ := args["org"].(string)

	opt, err := labelCreateFromArgs(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	var label labelDTO
	if err := forgejo.DoJSON(ctx, http.MethodPost, forgejo.APIPath("orgs", org, "labels"), opt, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("create org label: %w", err))
	}
	return to.TextResult(label)
}

func EditOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditOrgLabelFn")
	args := req.GetArguments()
	org, _ := args["org"].(string)
	idF, _ := to.Float64(args["id"])

	opt, err := labelPatchFromArgs(args, EditOrgLabelToolName)
	if err != nil {
		return to.ErrorResult(err)
	}

	var label labelDTO
	if err := forgejo.DoJSON(ctx, http.MethodPatch, forgejo.APIPath("orgs", org, "labels", int64(idF)), opt, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("edit org label: %w", err))
	}
	return to.TextResult(label)
}

func DeleteOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])
	deleteMode, _ := req.GetArguments()["delete_mode"].(string)
	id := int64(idF)

	if deleteMode != "force" {
		var label labelDTO
		if err := forgejo.DoJSON(ctx, http.MethodGet, forgejo.APIPath("orgs", org, "labels", id), nil, &label); err != nil {
			return to.ErrorResult(fmt.Errorf("get org label: %w", err))
		}

		client, err := forgejo.Client(ctx)
		if err != nil {
			return to.ErrorResult(err)
		}

		count, countErr := orgLabelInUseCount(ctx, client, org, label.Name)
		if countErr != nil {
			return to.ErrorResult(fmt.Errorf("label %q in-use count failed (token may lack org-repo access); use delete_mode=force to override: %w", label.Name, countErr))
		}
		if count > 0 {
			return to.ErrorResult(fmt.Errorf("label %q is used by %d issue(s)/PR(s) in visible repos (count excludes inaccessible repos); set delete_mode=force to delete anyway", label.Name, count))
		}
	}

	if err := forgejo.DoJSON(ctx, http.MethodDelete, forgejo.APIPath("orgs", org, "labels", id), nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete org label: %w", err))
	}
	return to.TextResult(map[string]any{"deleted": true, "id": id})
}

func GetOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	var label labelDTO
	if err := forgejo.DoJSON(ctx, http.MethodGet, forgejo.APIPath("orgs", org, "labels", int64(idF)), nil, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("get org label: %w", err))
	}
	return to.TextResult(label)
}
