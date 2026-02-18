package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListWorkflowRunsToolName = "list_workflow_runs"
	GetWorkflowRunToolName   = "get_workflow_run"
)

var (
	ListWorkflowRunsTool = mcp.NewTool(
		ListWorkflowRunsToolName,
		mcp.WithDescription("List workflow runs for a repository"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("status", mcp.Description(params.Status)),
		mcp.WithString("event", mcp.Description(params.Event)),
		mcp.WithNumber("run_number", mcp.Description(params.RunNumber)),
		mcp.WithString("head_sha", mcp.Description(params.HeadSHA)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(30), mcp.Min(1)),
	)

	GetWorkflowRunTool = mcp.NewTool(
		GetWorkflowRunToolName,
		mcp.WithDescription("Get details of a specific workflow run"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID)),
	)
)

func ListWorkflowRunsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListWorkflowRunsFn")

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}

	status, _ := req.GetArguments()["status"].(string)
	event, _ := req.GetArguments()["event"].(string)
	headSHA, _ := req.GetArguments()["head_sha"].(string)

	var runNumber int64
	if rn, ok := req.GetArguments()["run_number"].(float64); ok {
		runNumber = int64(rn)
	}

	page := 1
	if p, ok := req.GetArguments()["page"].(float64); ok {
		page = int(p)
	}
	limit := 30
	if l, ok := req.GetArguments()["limit"].(float64); ok {
		limit = int(l)
	}

	opt := forgejo_sdk.ListActionRunsOption{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     page,
			PageSize: limit,
		},
		Status:    status,
		Event:     event,
		RunNumber: runNumber,
		HeadSHA:   headSHA,
	}

	client := forgejo.Client()
	resp, _, err := client.ListRepoActionRuns(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("failed to list workflow runs: %w", err))
	}

	if len(resp.WorkflowRuns) == 0 {
		return to.TextResult(fmt.Sprintf("No workflow runs found for %s/%s", owner, repo))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Workflow Runs for %s/%s (total: %d):\n\n", owner, repo, resp.TotalCount))

	for _, run := range resp.WorkflowRuns {
		duration := ""
		if !run.Started.IsZero() && !run.Stopped.IsZero() {
			duration = fmt.Sprintf(" | Duration: %s", run.Stopped.Sub(run.Started).Round(1e9).String())
		}

		sb.WriteString(fmt.Sprintf("#%d - %s\n", run.ID, run.Title))
		sb.WriteString(fmt.Sprintf("  Status: %s | Event: %s | SHA: %.7s%s\n",
			run.Status, run.Event, run.CommitSHA, duration))
		if !run.Started.IsZero() {
			sb.WriteString(fmt.Sprintf("  Started: %s\n", run.Started.Format("2006-01-02 15:04:05")))
		}
		sb.WriteString(fmt.Sprintf("  URL: %s\n\n", run.HTMLURL))
	}

	return to.TextResult(sb.String())
}

func GetWorkflowRunFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetWorkflowRunFn")

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	runIDFloat, ok := req.GetArguments()["run_id"].(float64)
	if !ok {
		return to.ErrorResult(errors.New("run_id is required"))
	}
	runID := int64(runIDFloat)
	if runID <= 0 {
		return to.ErrorResult(errors.New("run_id must be a positive integer"))
	}

	client := forgejo.Client()
	run, _, err := client.GetRepoActionRun(owner, repo, runID)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("failed to get workflow run: %w", err))
	}

	duration := "N/A"
	if !run.Started.IsZero() && !run.Stopped.IsZero() {
		duration = run.Stopped.Sub(run.Started).Round(1e9).String()
	}

	triggerUser := "unknown"
	if run.TriggerUser != nil {
		triggerUser = run.TriggerUser.UserName
	}

	createdStr := "N/A"
	if !run.Created.IsZero() {
		createdStr = run.Created.Format("2006-01-02 15:04:05")
	}

	startedStr := "Not started"
	if !run.Started.IsZero() {
		startedStr = run.Started.Format("2006-01-02 15:04:05")
	}

	stoppedStr := "Not stopped"
	if !run.Stopped.IsZero() {
		stoppedStr = run.Stopped.Format("2006-01-02 15:04:05")
	}

	result := fmt.Sprintf(`Workflow Run #%d

Title: %s
Status: %s
Event: %s
Ref: %s
Commit SHA: %s
Triggered by: %s

Created: %s
Started: %s
Stopped: %s
Duration: %s

URL: %s`,
		run.ID,
		run.Title,
		run.Status,
		run.Event,
		run.PrettyRef,
		run.CommitSHA,
		triggerUser,
		createdStr,
		startedStr,
		stoppedStr,
		duration,
		run.HTMLURL,
	)

	return to.TextResult(result)
}
