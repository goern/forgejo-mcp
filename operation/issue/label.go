// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/ptr"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

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
		mcp.WithDescription("Create a repository label. Returns the created label including its numeric id."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("Label description")),
	)

	EditRepoLabelTool = mcp.NewTool(
		EditRepoLabelToolName,
		mcp.WithDescription("Edit a repository label (PATCH — only supplied fields change). Providing no fields is an error."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("New label description")),
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
		mcp.WithDescription("Get a single repository label by ID."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
	)

	CreateOrgLabelTool = mcp.NewTool(
		CreateOrgLabelToolName,
		mcp.WithDescription("Create an organization-level label. Returns the created label including its numeric id."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("Label description")),
	)

	EditOrgLabelTool = mcp.NewTool(
		EditOrgLabelToolName,
		mcp.WithDescription("Edit an organization-level label (PATCH — only supplied fields change). Providing no fields is an error."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("New label description")),
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
		mcp.WithDescription("Get a single organization-level label by ID."),
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
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			return "", fmt.Errorf("color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got %q", color)
		}
	}
	return "#" + c, nil
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

// ---- Org label raw-HTTP types ----

type orgLabelOption struct {
	Name        string  `json:"name,omitempty"`
	Color       string  `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

type orgLabelPatchOption struct {
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ---- Repo label handlers ----

func CreateRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	name, _ := req.GetArguments()["name"].(string)
	colorRaw, _ := req.GetArguments()["color"].(string)
	description, _ := req.GetArguments()["description"].(string)

	color, err := normalizeColor(colorRaw)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.CreateLabelOption{
		Name:        name,
		Color:       color,
		Description: description,
	}
	label, _, err := client.CreateLabel(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create repo label: %w", err))
	}
	return to.TextResult(label)
}

func EditRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	nameRaw, nameSet := req.GetArguments()["name"].(string)
	colorRaw, colorSet := req.GetArguments()["color"].(string)
	descRaw, descSet := req.GetArguments()["description"].(string)

	if !nameSet && !colorSet && !descSet {
		return to.ErrorResult(fmt.Errorf("edit_repo_label: at least one of name, color, description must be provided"))
	}

	opt := forgejo_sdk.EditLabelOption{}
	if nameSet && nameRaw != "" {
		opt.Name = ptr.To(nameRaw)
	}
	if colorSet && colorRaw != "" {
		color, err := normalizeColor(colorRaw)
		if err != nil {
			return to.ErrorResult(err)
		}
		opt.Color = ptr.To(color)
	}
	if descSet {
		opt.Description = ptr.To(descRaw)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	label, _, err := client.EditLabel(owner, repo, int64(idF), opt)
	if err != nil {
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

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	label, _, err := client.GetRepoLabel(owner, repo, int64(idF))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get repo label: %w", err))
	}
	return to.TextResult(label)
}

// ---- Org label handlers ----

func CreateOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	name, _ := req.GetArguments()["name"].(string)
	colorRaw, _ := req.GetArguments()["color"].(string)
	description, _ := req.GetArguments()["description"].(string)

	color, err := normalizeColor(colorRaw)
	if err != nil {
		return to.ErrorResult(err)
	}

	body := orgLabelOption{Name: name, Color: color}
	if description != "" {
		body.Description = ptr.To(description)
	}

	var label forgejo_sdk.Label
	if err := forgejo.DoJSON(ctx, http.MethodPost, fmt.Sprintf("/orgs/%s/labels", org), body, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("create org label: %w", err))
	}
	return to.TextResult(label)
}

func EditOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	nameRaw, nameSet := req.GetArguments()["name"].(string)
	colorRaw, colorSet := req.GetArguments()["color"].(string)
	descRaw, descSet := req.GetArguments()["description"].(string)

	if !nameSet && !colorSet && !descSet {
		return to.ErrorResult(fmt.Errorf("edit_org_label: at least one of name, color, description must be provided"))
	}

	opt := orgLabelPatchOption{}
	if nameSet && nameRaw != "" {
		opt.Name = ptr.To(nameRaw)
	}
	if colorSet && colorRaw != "" {
		color, err := normalizeColor(colorRaw)
		if err != nil {
			return to.ErrorResult(err)
		}
		opt.Color = ptr.To(color)
	}
	if descSet {
		opt.Description = ptr.To(descRaw)
	}

	var label forgejo_sdk.Label
	if err := forgejo.DoJSON(ctx, http.MethodPatch, fmt.Sprintf("/orgs/%s/labels/%d", org, int64(idF)), opt, &label); err != nil {
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
		var label forgejo_sdk.Label
		if err := forgejo.DoJSON(ctx, http.MethodGet, fmt.Sprintf("/orgs/%s/labels/%d", org, id), nil, &label); err != nil {
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

	if err := forgejo.DoJSON(ctx, http.MethodDelete, fmt.Sprintf("/orgs/%s/labels/%d", org, id), nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete org label: %w", err))
	}
	return to.TextResult(map[string]any{"deleted": true, "id": id})
}

func GetOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	var label forgejo_sdk.Label
	if err := forgejo.DoJSON(ctx, http.MethodGet, fmt.Sprintf("/orgs/%s/labels/%d", org, int64(idF)), nil, &label); err != nil {
		return to.ErrorResult(fmt.Errorf("get org label: %w", err))
	}
	return to.TextResult(label)
}
