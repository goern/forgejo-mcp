// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"fmt"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	CancelWorkflowRunToolName = "cancel_workflow_run"
	DeleteWorkflowRunToolName = "delete_workflow_run"
)

type workflowRunControlResult struct {
	RunID  int64  `json:"run_id"`
	Status string `json:"status"`
}

var (
	CancelWorkflowRunTool = mcp.NewTool(
		CancelWorkflowRunToolName,
		mcp.WithDescription("Cancel a pending or running workflow run. Forgejo also returns 204 for a run that has already finished and leaves that run unchanged; that is success, not an error. Same Actions write as dispatch_workflow; a read-only token returns 403."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
	)

	DeleteWorkflowRunTool = mcp.NewTool(
		DeleteWorkflowRunToolName,
		mcp.WithDescription("Delete a completed workflow run (succeeded, failed, or cancelled). The run is removed, its job logs become unreadable, and Forgejo marks that run's artifacts deleted (storage reclaimed later). A live run is an API error (never mapped to success). No preflight GET. Same Actions write as dispatch_workflow; a read-only token returns 403."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
	)
)

func CancelWorkflowRunFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CancelWorkflowRunFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(args, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := forgejo.APIPath("repos", owner, repo, "actions", "runs", runID, "cancel")
	if err := forgejo.DoJSON(ctx, http.MethodPost, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("cancel workflow run: %w", err))
	}
	return to.TextResult(workflowRunControlResult{RunID: runID, Status: "cancelled"})
}

func DeleteWorkflowRunFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteWorkflowRunFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(args, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := forgejo.APIPath("repos", owner, repo, "actions", "runs", runID)
	if err := forgejo.DoJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return to.ErrorResult(fmt.Errorf("delete workflow run: %w", err))
	}
	return to.TextResult(workflowRunControlResult{RunID: runID, Status: "deleted"})
}
