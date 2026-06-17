// SPDX-License-Identifier: GPL-3.0-or-later

// Package hook provides MCP tools and resources to read and manage
// Forgejo/Codeberg repository webhooks (/repos/{owner}/{repo}/hooks).
package hook

import (
	"context"
	"fmt"
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
	ListRepoHooksToolName   = "list_repo_hooks"
	GetRepoHookToolName     = "get_repo_hook"
	CreateRepoHookToolName  = "create_repo_hook"
	EditRepoHookToolName    = "edit_repo_hook"
	DeleteRepoHookToolName  = "delete_repo_hook"
	TestRepoHookToolName    = "test_repo_hook"
)

var (
	ListRepoHooksTool = mcp.NewTool(
		ListRepoHooksToolName,
		mcp.WithDescription("List repository webhooks (bounded by page/limit, default 30 per page)"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(30), mcp.Min(1)),
	)

	GetRepoHookTool = mcp.NewTool(
		GetRepoHookToolName,
		mcp.WithDescription("Get a single repository webhook by ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID")),
	)

	CreateRepoHookTool = mcp.NewTool(
		CreateRepoHookToolName,
		mcp.WithDescription("Create a repository webhook. The secret is accepted but never echoed in the response."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("url", mcp.Required(), mcp.Description("Payload URL for the webhook")),
		mcp.WithString("content_type", mcp.Description(`Content type: "json" or "form" (default "json")`)),
		mcp.WithString("secret", mcp.Description("HMAC secret for payload signing (write-only; not returned)")),
		mcp.WithString("http_method", mcp.Description(`HTTP method: "POST" or "GET" (default "POST")`)),
		mcp.WithString("branch_filter", mcp.Description("Branch filter glob (e.g. main, release/*)")),
		mcp.WithString("events", mcp.Description(`Webhook events (comma-separated, e.g. "push,pull_request")`)),
		mcp.WithBoolean("active", mcp.Description("Whether the webhook is active (default true)")),
	)

	EditRepoHookTool = mcp.NewTool(
		EditRepoHookToolName,
		mcp.WithDescription("Edit a repository webhook. Only fields you pass are changed; omitted fields are left untouched."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID")),
		mcp.WithString("url", mcp.Description("New payload URL")),
		mcp.WithString("content_type", mcp.Description(`Content type: "json" or "form"`)),
		mcp.WithString("secret", mcp.Description("New HMAC secret (write-only; not returned)")),
		mcp.WithString("http_method", mcp.Description(`HTTP method: "POST" or "GET"`)),
		mcp.WithString("branch_filter", mcp.Description("Branch filter glob")),
		mcp.WithString("events", mcp.Description(`Webhook events (comma-separated)`)),
		mcp.WithBoolean("active", mcp.Description("Whether the webhook is active")),
	)

	DeleteRepoHookTool = mcp.NewTool(
		DeleteRepoHookToolName,
		mcp.WithDescription("Delete a repository webhook by ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID")),
	)

	TestRepoHookTool = mcp.NewTool(
		TestRepoHookToolName,
		mcp.WithDescription("Trigger a test delivery for a repository webhook. WARNING: each call triggers a live HTTP delivery to the webhook URL."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID")),
	)
)

// RegisterTools registers all repository webhook tools on the MCP server.
func RegisterTools(s *server.MCPServer) {
	s.AddTool(ListRepoHooksTool, ListRepoHooksFn)
	s.AddTool(GetRepoHookTool, GetRepoHookFn)
	s.AddTool(CreateRepoHookTool, CreateRepoHookFn)
	s.AddTool(EditRepoHookTool, EditRepoHookFn)
	s.AddTool(DeleteRepoHookTool, DeleteRepoHookFn)
	s.AddTool(TestRepoHookTool, TestRepoHookFn)
}

// hookPayload is the safe MCP response — Config.secret is never included.
// D3: explicit allowlist of config keys copied individually from Hook.Config.
type hookPayload struct {
	ID           int64    `json:"id"`
	Type         string   `json:"type"`
	Active       bool     `json:"active"`
	Events       []string `json:"events"`
	URL          string   `json:"url"`
	ContentType  string   `json:"content_type,omitempty"`
	HTTPMethod   string   `json:"http_method,omitempty"`
	BranchFilter string   `json:"branch_filter,omitempty"`
}

func safeHook(h *forgejo_sdk.Hook) hookPayload {
	return hookPayload{
		ID:           h.ID,
		Type:         h.Type,
		Active:       h.Active,
		Events:       h.Events,
		URL:          h.Config["url"],
		ContentType:  h.Config["content_type"],
		HTTPMethod:   h.Config["http_method"],
		BranchFilter: h.Config["branch_filter"],
	}
}

type listRepoHooksResult struct {
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
	Count int           `json:"count"`
	Hooks []hookPayload `json:"hooks"`
}

func ListRepoHooksFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoHooksFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 30
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	hooks, _, err := client.ListRepoHooks(owner, repo, forgejo_sdk.ListHooksOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: int(page), PageSize: int(limit)},
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo hooks: %w", err))
	}
	payloads := make([]hookPayload, len(hooks))
	for i, h := range hooks {
		payloads[i] = safeHook(h)
	}
	return to.TextResult(listRepoHooksResult{
		Page:  int(page),
		Limit: int(limit),
		Count: len(payloads),
		Hooks: payloads,
	})
}

func GetRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetRepoHookFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	idF, _ := to.Float64(args["id"])
	if owner == "" || repo == "" || idF == 0 {
		return to.ErrorResult(fmt.Errorf("owner, repo and id are required"))
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	h, _, err := client.GetRepoHook(owner, repo, int64(idF))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get repo hook: %w", err))
	}
	return to.TextResult(safeHook(h))
}

func CreateRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateRepoHookFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	hookURL, _ := args["url"].(string)
	if owner == "" || repo == "" || hookURL == "" {
		return to.ErrorResult(fmt.Errorf("owner, repo and url are required"))
	}

	config := map[string]string{
		"url":          hookURL,
		"content_type": "json",
	}
	if v, ok := args["content_type"].(string); ok && v != "" {
		config["content_type"] = v
	}
	if v, ok := args["secret"].(string); ok && v != "" {
		config["secret"] = v
	}
	if v, ok := args["http_method"].(string); ok && v != "" {
		config["http_method"] = v
	}

	opt := forgejo_sdk.CreateHookOption{
		Type:   forgejo_sdk.HookTypeForgejo,
		Config: config,
		Active: true,
	}
	if v, ok := args["events"].(string); ok && v != "" {
		opt.Events = splitEvents(v)
	}
	if v, ok := args["branch_filter"].(string); ok {
		opt.BranchFilter = v
	}
	if v, ok := args["active"].(bool); ok {
		opt.Active = v
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	h, _, err := client.CreateRepoHook(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create repo hook: %w", err))
	}
	return to.TextResult(safeHook(h))
}

func EditRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditRepoHookFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	idF, _ := to.Float64(args["id"])
	if owner == "" || repo == "" || idF == 0 {
		return to.ErrorResult(fmt.Errorf("owner, repo and id are required"))
	}

	// Fetch existing hook to merge config (PATCH semantics for config map).
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	existing, _, err := client.GetRepoHook(owner, repo, int64(idF))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get repo hook for edit: %w", err))
	}

	config := existing.Config
	if v, ok := args["url"].(string); ok && v != "" {
		config["url"] = v
	}
	if v, ok := args["content_type"].(string); ok && v != "" {
		config["content_type"] = v
	}
	if v, ok := args["secret"].(string); ok && v != "" {
		config["secret"] = v
	}
	if v, ok := args["http_method"].(string); ok && v != "" {
		config["http_method"] = v
	}

	opt := forgejo_sdk.EditHookOption{
		Config: config,
	}
	if v, ok := args["events"].(string); ok && v != "" {
		opt.Events = splitEvents(v)
	}
	if v, ok := args["branch_filter"].(string); ok {
		opt.BranchFilter = v
	}
	if v, ok := args["active"].(bool); ok {
		opt.Active = ptr.To(v)
	}

	resp, err := client.EditRepoHook(owner, repo, int64(idF), opt)
	if err != nil {
		if resp != nil {
			return to.ErrorResult(fmt.Errorf("edit repo hook %d %s", resp.StatusCode, err.Error()))
		}
		return to.ErrorResult(fmt.Errorf("edit repo hook: %w", err))
	}

	// Re-fetch to return the updated state.
	h, _, err := client.GetRepoHook(owner, repo, int64(idF))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get repo hook after edit: %w", err))
	}
	return to.TextResult(safeHook(h))
}

func DeleteRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteRepoHookFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	idF, _ := to.Float64(args["id"])
	if owner == "" || repo == "" || idF == 0 {
		return to.ErrorResult(fmt.Errorf("owner, repo and id are required"))
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.DeleteRepoHook(owner, repo, int64(idF)); err != nil {
		return to.ErrorResult(fmt.Errorf("delete repo hook: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Deleted webhook %d on %s/%s", int64(idF), owner, repo))
}

func TestRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called TestRepoHookFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	idF, _ := to.Float64(args["id"])
	if owner == "" || repo == "" || idF == 0 {
		return to.ErrorResult(fmt.Errorf("owner, repo and id are required"))
	}

	// TestRepoHook is not in SDK v3; call the API directly (POST /repos/{owner}/{repo}/hooks/{id}/tests).
	path := fmt.Sprintf("/repos/%s/%s/hooks/%d/tests", owner, repo, int64(idF))
	if err := forgejo.DoJSON(ctx, "POST", path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("test repo hook: %w", err))
	}
	return to.TextResult(map[string]bool{"triggered": true})
}

func splitEvents(csv string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
