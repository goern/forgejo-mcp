package actions

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListWorkflowRunsToolName    = "list_workflow_runs"
	GetWorkflowRunToolName      = "get_workflow_run"
	ListWorkflowRunJobsToolName = "list_workflow_run_jobs"
	GetWorkflowJobLogsToolName  = "get_workflow_job_logs"
	GetWorkflowRunLogsToolName  = "get_workflow_run_logs"
)

type actionRunJob struct {
	ID      int64    `json:"id"`
	RunID   int64    `json:"run_id"`
	Attempt int64    `json:"attempt"`
	Handle  string   `json:"handle"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	TaskID  int64    `json:"task_id"`
	OwnerID int64    `json:"owner_id"`
	RepoID  int64    `json:"repo_id"`
	RunsOn  []string `json:"runs_on"`
	Needs   []string `json:"needs"`
}

type workflowRunLogsResult struct {
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	Encoding      string `json:"encoding"`
	ContentBase64 string `json:"content_base64"`
	Bytes         int    `json:"bytes"`
}

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

	ListWorkflowRunJobsTool = mcp.NewTool(
		ListWorkflowRunJobsToolName,
		mcp.WithDescription("List jobs for a workflow run, including per-job status, attempt, task ID, runner labels, and log tool hints."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID)),
	)

	GetWorkflowJobLogsTool = mcp.NewTool(
		GetWorkflowJobLogsToolName,
		mcp.WithDescription("Download plaintext logs for a single workflow job. Omit attempt for the latest attempt; use tail_lines to return only the end."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("job_id", mcp.Required(), mcp.Description(params.JobID)),
		mcp.WithNumber("attempt", mcp.Description(params.Attempt)),
		mcp.WithNumber("tail_lines", mcp.Description("Return only the last N log lines"), mcp.Min(1)),
	)

	GetWorkflowRunLogsTool = mcp.NewTool(
		GetWorkflowRunLogsToolName,
		mcp.WithDescription("Download a ZIP archive containing plaintext logs for every job in a workflow run. Returns base64 content when under the inline cap."),
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
	if rn, err := to.Float64(req.GetArguments()["run_number"]); err == nil {
		runNumber = int64(rn)
	}

	page := 1
	if p, err := to.Float64(req.GetArguments()["page"]); err == nil {
		page = int(p)
	}
	limit := 30
	if l, err := to.Float64(req.GetArguments()["limit"]); err == nil {
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

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	resp, _, err := client.ListRepoActionRuns(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("failed to list workflow runs: %w", err))
	}

	if len(resp.WorkflowRuns) == 0 {
		return to.TextResult(fmt.Sprintf("No workflow runs found for %s/%s", owner, repo))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Workflow Runs for %s/%s (total: %d):\n\n", owner, repo, resp.TotalCount)

	for _, run := range resp.WorkflowRuns {
		duration := ""
		if !run.Started.IsZero() && !run.Stopped.IsZero() {
			duration = fmt.Sprintf(" | Duration: %s", run.Stopped.Sub(run.Started).Round(1e9).String())
		}

		fmt.Fprintf(&sb, "#%d - %s\n", run.ID, run.Title)
		fmt.Fprintf(&sb, "  Status: %s | Event: %s | SHA: %.7s%s\n",
			run.Status, run.Event, run.CommitSHA, duration)
		if !run.Started.IsZero() {
			fmt.Fprintf(&sb, "  Started: %s\n", run.Started.Format("2006-01-02 15:04:05"))
		}
		fmt.Fprintf(&sb, "  URL: %s\n\n", run.HTMLURL)
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
	runIDFloat, err := to.Float64(req.GetArguments()["run_id"])
	if err != nil {
		return to.ErrorResult(errors.New("run_id is required"))
	}
	runID := int64(runIDFloat)
	if runID <= 0 {
		return to.ErrorResult(errors.New("run_id must be a positive integer"))
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
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

func ListWorkflowRunJobsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListWorkflowRunJobsFn")

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	runIDFloat, err := to.Float64(req.GetArguments()["run_id"])
	if err != nil {
		return to.ErrorResult(errors.New("run_id is required"))
	}
	runID := int64(runIDFloat)
	if runID <= 0 {
		return to.ErrorResult(errors.New("run_id must be a positive integer"))
	}

	var jobs []*actionRunJob
	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", url.PathEscape(owner), url.PathEscape(repo), runID)
	if err := forgejo.DoJSON(ctx, "GET", path, nil, &jobs); err != nil {
		return to.ErrorResult(fmt.Errorf("failed to list workflow run jobs: %w", err))
	}
	if len(jobs) == 0 {
		return to.TextResult(fmt.Sprintf("No jobs found for workflow run %d in %s/%s", runID, owner, repo))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Jobs for workflow run %d in %s/%s:\n\n", runID, owner, repo)
	for _, job := range jobs {
		fmt.Fprintf(&sb, "#%d - %s\n", job.ID, job.Name)
		fmt.Fprintf(&sb, "  Status: %s | Attempt: %d | Task ID: %d\n", job.Status, job.Attempt, job.TaskID)
		if len(job.RunsOn) > 0 {
			fmt.Fprintf(&sb, "  Runs on: %s\n", strings.Join(job.RunsOn, ", "))
		}
		if len(job.Needs) > 0 {
			fmt.Fprintf(&sb, "  Needs: %s\n", strings.Join(job.Needs, ", "))
		}
		fmt.Fprintf(&sb, "  Logs: call %s with job_id=%d", GetWorkflowJobLogsToolName, job.ID)
		if job.Attempt > 0 {
			fmt.Fprintf(&sb, " (attempt=%d for this attempt)", job.Attempt)
		}
		fmt.Fprint(&sb, "\n\n")
	}

	return to.TextResult(sb.String())
}

func GetWorkflowJobLogsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetWorkflowJobLogsFn")

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	jobIDFloat, err := to.Float64(req.GetArguments()["job_id"])
	if err != nil {
		return to.ErrorResult(errors.New("job_id is required"))
	}
	jobID := int64(jobIDFloat)
	if jobID <= 0 {
		return to.ErrorResult(errors.New("job_id must be a positive integer"))
	}

	path := fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", url.PathEscape(owner), url.PathEscape(repo), jobID)
	if attemptFloat, err := to.Float64(req.GetArguments()["attempt"]); err == nil {
		attempt := int64(attemptFloat)
		if attempt <= 0 {
			return to.ErrorResult(errors.New("attempt must be a positive integer"))
		}
		path += fmt.Sprintf("?attempt=%d", attempt)
	}

	body, _, err := forgejo.DoAPIRaw(ctx, path)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("failed to get workflow job logs: %w", err))
	}
	text := string(body)
	if tailFloat, err := to.Float64(req.GetArguments()["tail_lines"]); err == nil {
		tailLines := int(tailFloat)
		if tailLines <= 0 {
			return to.ErrorResult(errors.New("tail_lines must be a positive integer"))
		}
		text = tail(text, tailLines)
	}

	return to.TextResult(text)
}

func GetWorkflowRunLogsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetWorkflowRunLogsFn")

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	runIDFloat, err := to.Float64(req.GetArguments()["run_id"])
	if err != nil {
		return to.ErrorResult(errors.New("run_id is required"))
	}
	runID := int64(runIDFloat)
	if runID <= 0 {
		return to.ErrorResult(errors.New("run_id must be a positive integer"))
	}

	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/logs", url.PathEscape(owner), url.PathEscape(repo), runID)
	body, contentType, err := forgejo.DoAPIRaw(ctx, path)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("failed to get workflow run logs: %w", err))
	}
	result := workflowRunLogsResult{
		Filename:      fmt.Sprintf("run-%d-logs.zip", runID),
		ContentType:   contentType,
		Encoding:      "base64",
		ContentBase64: base64.StdEncoding.EncodeToString(body),
		Bytes:         len(body),
	}

	return to.SafeTextResult(result)
}

func tail(text string, lines int) string {
	parts := strings.SplitAfter(text, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if lines >= len(parts) {
		return text
	}
	return strings.Join(parts[len(parts)-lines:], "")
}
