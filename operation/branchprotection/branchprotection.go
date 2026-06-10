// Package branchprotection provides MCP tools and resources to read and manage
// Forgejo/Codeberg branch protection rules
// (/repos/{owner}/{repo}/branch_protections). Motivated by forgejo-mcp-f6h:
// a repo with no protection let Renovate automerge before CI was green.
package branchprotection

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
	ListBranchProtectionsToolName  = "list_branch_protections"
	GetBranchProtectionToolName    = "get_branch_protection"
	CreateBranchProtectionToolName = "create_branch_protection"
	EditBranchProtectionToolName   = "edit_branch_protection"
	DeleteBranchProtectionToolName = "delete_branch_protection"
)

var (
	ListBranchProtectionsTool = mcp.NewTool(
		ListBranchProtectionsToolName,
		mcp.WithDescription("List a repository's branch protection rules (bounded by page/limit)"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	GetBranchProtectionTool = mcp.NewTool(
		GetBranchProtectionToolName,
		mcp.WithDescription("Get a single branch protection rule by name"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("rule", mcp.Required(), mcp.Description(params.BPRule)),
	)

	CreateBranchProtectionTool = mcp.NewTool(
		CreateBranchProtectionToolName,
		mcp.WithDescription("Create a branch protection rule (e.g. require status checks before merge)"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.Branch)),
		mcp.WithString("rule_name", mcp.Description(params.BPRuleName)),
		mcp.WithBoolean("enable_push", mcp.Description(params.BPEnablePush)),
		mcp.WithBoolean("enable_status_check", mcp.Description(params.BPEnableStatusCheck)),
		mcp.WithString("status_check_contexts", mcp.Description(params.BPStatusCheckContexts)),
		mcp.WithNumber("required_approvals", mcp.Description(params.BPRequiredApprovals), mcp.Min(0)),
		mcp.WithBoolean("block_on_outdated_branch", mcp.Description(params.BPBlockOnOutdatedBranch)),
		mcp.WithBoolean("require_signed_commits", mcp.Description(params.BPRequireSignedCommits)),
		mcp.WithBoolean("dismiss_stale_approvals", mcp.Description(params.BPDismissStaleApprovals)),
	)

	EditBranchProtectionTool = mcp.NewTool(
		EditBranchProtectionToolName,
		mcp.WithDescription("Edit a branch protection rule. Only fields you pass are changed; omitted fields are left untouched."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("rule", mcp.Required(), mcp.Description(params.BPRule)),
		mcp.WithBoolean("enable_push", mcp.Description(params.BPEnablePush)),
		mcp.WithBoolean("enable_status_check", mcp.Description(params.BPEnableStatusCheck)),
		mcp.WithString("status_check_contexts", mcp.Description(params.BPStatusCheckContexts)),
		mcp.WithNumber("required_approvals", mcp.Description(params.BPRequiredApprovals), mcp.Min(0)),
		mcp.WithBoolean("block_on_outdated_branch", mcp.Description(params.BPBlockOnOutdatedBranch)),
		mcp.WithBoolean("require_signed_commits", mcp.Description(params.BPRequireSignedCommits)),
		mcp.WithBoolean("dismiss_stale_approvals", mcp.Description(params.BPDismissStaleApprovals)),
		mcp.WithBoolean("enable_push_whitelist", mcp.Description(params.BPEnablePushWhitelist)),
		mcp.WithString("push_whitelist_usernames", mcp.Description(params.BPPushWhitelistUsers)),
		mcp.WithBoolean("enable_merge_whitelist", mcp.Description(params.BPEnableMergeWhitelist)),
		mcp.WithString("merge_whitelist_usernames", mcp.Description(params.BPMergeWhitelistUsers)),
		mcp.WithBoolean("enable_approvals_whitelist", mcp.Description(params.BPEnableApprovalsWl)),
		mcp.WithString("approvals_whitelist_usernames", mcp.Description(params.BPApprovalsWhitelistUsers)),
	)

	DeleteBranchProtectionTool = mcp.NewTool(
		DeleteBranchProtectionToolName,
		mcp.WithDescription("Delete a branch protection rule by name"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("rule", mcp.Required(), mcp.Description(params.BPRule)),
	)
)

// RegisterTool registers the branch protection CRUD tools on the MCP server.
func RegisterTool(s *server.MCPServer) {
	s.AddTool(ListBranchProtectionsTool, ListBranchProtectionsFn)
	s.AddTool(GetBranchProtectionTool, GetBranchProtectionFn)
	s.AddTool(CreateBranchProtectionTool, CreateBranchProtectionFn)
	s.AddTool(EditBranchProtectionTool, EditBranchProtectionFn)
	s.AddTool(DeleteBranchProtectionTool, DeleteBranchProtectionFn)
}

// listBranchProtectionsResult wraps the rules with the page echo so callers can
// resume pagination (output-bounding sub-rule 3).
type listBranchProtectionsResult struct {
	Page              int                             `json:"page"`
	Limit             int                             `json:"limit"`
	Count             int                             `json:"count"`
	BranchProtections []*forgejo_sdk.BranchProtection `json:"branch_protections"`
}

func ListBranchProtectionsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListBranchProtectionsFn")
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
		limit = 100
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	bps, _, err := client.ListBranchProtections(owner, repo, forgejo_sdk.ListBranchProtectionsOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: int(page), PageSize: int(limit)},
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list branch protections err: %w", err))
	}
	return to.TextResult(listBranchProtectionsResult{
		Page:              int(page),
		Limit:             int(limit),
		Count:             len(bps),
		BranchProtections: bps,
	})
}

func GetBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetBranchProtectionFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rule, _ := args["rule"].(string)
	if owner == "" || repo == "" || rule == "" {
		return to.ErrorResult(fmt.Errorf("owner, repo and rule are required"))
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	bp, _, err := client.GetBranchProtection(owner, repo, rule)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get branch protection err: %w", err))
	}
	return to.TextResult(bp)
}

func CreateBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateBranchProtectionFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}
	branchName, _ := args["branch_name"].(string)
	if branchName == "" {
		return to.ErrorResult(fmt.Errorf("branch_name is required"))
	}

	opt := forgejo_sdk.CreateBranchProtectionOption{BranchName: branchName}
	if v, ok := args["rule_name"].(string); ok {
		opt.RuleName = v
	}
	if v, ok := args["enable_push"].(bool); ok {
		opt.EnablePush = v
	}
	if v, ok := args["enable_status_check"].(bool); ok {
		opt.EnableStatusCheck = v
	}
	if v, ok := args["status_check_contexts"].(string); ok {
		opt.StatusCheckContexts = splitContexts(v)
	}
	if v, ok := to.Float64Ok(args["required_approvals"]); ok {
		opt.RequiredApprovals = int64(v)
	}
	if v, ok := args["block_on_outdated_branch"].(bool); ok {
		opt.BlockOnOutdatedBranch = v
	}
	if v, ok := args["require_signed_commits"].(bool); ok {
		opt.RequireSignedCommits = v
	}
	if v, ok := args["dismiss_stale_approvals"].(bool); ok {
		opt.DismissStaleApprovals = v
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	bp, _, err := client.CreateBranchProtection(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create branch protection err: %w", err))
	}
	return to.TextResult(bp)
}

func EditBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditBranchProtectionFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rule, _ := args["rule"].(string)
	if owner == "" || repo == "" || rule == "" {
		return to.ErrorResult(fmt.Errorf("owner, repo and rule are required"))
	}

	// PATCH semantics: only set a field when the caller supplied it, so omitted
	// fields are left unchanged server-side.
	var opt forgejo_sdk.EditBranchProtectionOption
	if v, ok := args["enable_push"].(bool); ok {
		opt.EnablePush = ptr.To(v)
	}
	if v, ok := args["enable_status_check"].(bool); ok {
		opt.EnableStatusCheck = ptr.To(v)
	}
	if v, ok := args["status_check_contexts"].(string); ok {
		opt.StatusCheckContexts = splitContexts(v)
	}
	if v, ok := to.Float64Ok(args["required_approvals"]); ok {
		opt.RequiredApprovals = ptr.To(int64(v))
	}
	if v, ok := args["block_on_outdated_branch"].(bool); ok {
		opt.BlockOnOutdatedBranch = ptr.To(v)
	}
	if v, ok := args["require_signed_commits"].(bool); ok {
		opt.RequireSignedCommits = ptr.To(v)
	}
	if v, ok := args["dismiss_stale_approvals"].(bool); ok {
		opt.DismissStaleApprovals = ptr.To(v)
	}
	if v, ok := args["enable_push_whitelist"].(bool); ok {
		opt.EnablePushWhitelist = ptr.To(v)
	}
	if v, ok := args["push_whitelist_usernames"].(string); ok {
		opt.PushWhitelistUsernames = splitContexts(v)
	}
	if v, ok := args["enable_merge_whitelist"].(bool); ok {
		opt.EnableMergeWhitelist = ptr.To(v)
	}
	if v, ok := args["merge_whitelist_usernames"].(string); ok {
		opt.MergeWhitelistUsernames = splitContexts(v)
	}
	if v, ok := args["enable_approvals_whitelist"].(bool); ok {
		opt.EnableApprovalsWhitelist = ptr.To(v)
	}
	if v, ok := args["approvals_whitelist_usernames"].(string); ok {
		opt.ApprovalsWhitelistUsernames = splitContexts(v)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	bp, _, err := client.EditBranchProtection(owner, repo, rule, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit branch protection err: %w", err))
	}
	return to.TextResult(bp)
}

func DeleteBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteBranchProtectionFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rule, _ := args["rule"].(string)
	if owner == "" || repo == "" || rule == "" {
		return to.ErrorResult(fmt.Errorf("owner, repo and rule are required"))
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.DeleteBranchProtection(owner, repo, rule); err != nil {
		return to.ErrorResult(fmt.Errorf("delete branch protection err: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Deleted branch protection rule %q on %s/%s", rule, owner, repo))
}

// splitContexts parses a comma-separated status-check context list into a
// trimmed, empty-free slice. Returns an empty (non-nil) slice for "".
func splitContexts(csv string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
