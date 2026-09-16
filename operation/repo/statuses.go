// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetCommitStatusesToolName = "get_commit_statuses"

	defaultCommitStatusesLimit = resource.EmbeddedListCap
	maxCommitStatusesLimit     = 50
)

var (
	GetCommitStatusesTool = mcp.NewTool(
		GetCommitStatusesToolName,
		mcp.WithDescription("List per-context commit statuses for a full 40-character SHA. Bounded by page (default 1) and limit (default 30, maximum 50); returns {sha, statuses, page, limit, count, total_count}. total_count is present only when Forgejo reports X-Total-Count. Combined aggregate stays on forgejo://repo/{owner}/{repo}/commit/{sha}/status. Commit statuses are CI context checks, not Actions workflow runs (list_workflow_runs)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("sha", mcp.Required(), mcp.Description("Full 40-character hex commit SHA")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultCommitStatusesLimit), mcp.Min(1), mcp.Max(maxCommitStatusesLimit)),
	)
)

type getCommitStatusesResult struct {
	SHA        string       `json:"sha"`
	Statuses   []statusItem `json:"statuses"`
	Page       int          `json:"page"`
	Limit      int          `json:"limit"`
	Count      int          `json:"count"`
	TotalCount *int         `json:"total_count,omitempty"`
}

func GetCommitStatusesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetCommitStatusesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	sha, _ := args["sha"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}
	if err := resource.ValidateSHA(sha); err != nil {
		return to.ErrorResult(err)
	}

	page, _ := to.Float64(args["page"])
	if page < 1 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit < 1 {
		limit = float64(defaultCommitStatusesLimit)
	}
	if limit > float64(maxCommitStatusesLimit) {
		limit = float64(maxCommitStatusesLimit)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	statuses, resp, err := client.ListStatuses(owner, repo, sha, forgejo_sdk.ListStatusesOption{
		ListOptions: forgejo_sdk.ListOptions{Page: int(page), PageSize: int(limit)},
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get commit statuses: %w", err))
	}

	items := make([]statusItem, 0, len(statuses))
	for _, s := range statuses {
		item, ok := toStatusItem(s)
		if !ok {
			continue
		}
		items = append(items, item)
	}

	var totalCount *int
	if resp != nil {
		totalCount = forgejo.TotalCountPtr(resp.Header)
	}

	return to.TextResult(getCommitStatusesResult{
		SHA:        sha,
		Statuses:   items,
		Page:       int(page),
		Limit:      int(limit),
		Count:      len(items),
		TotalCount: totalCount,
	})
}
