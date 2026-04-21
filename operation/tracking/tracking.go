package tracking

import (
	"context"
	"fmt"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListIssueTrackedTimesToolName  = "list_issue_tracked_times"
	ListRepoTrackedTimesToolName   = "list_repo_tracked_times"
	ListMyTrackedTimesToolName     = "list_my_tracked_times"
	AddIssueTimeToolName           = "add_issue_time"
	ResetIssueTimeToolName         = "reset_issue_time"
	DeleteIssueTimeEntryToolName   = "delete_issue_time_entry"
	StartIssueStopwatchToolName    = "start_issue_stopwatch"
	StopIssueStopwatchToolName     = "stop_issue_stopwatch"
	CancelIssueStopwatchToolName   = "cancel_issue_stopwatch"
	ListMyStopwatchesToolName      = "list_my_stopwatches"

	indexDescIssueOrPR = "Issue or pull request index (Forgejo shares index namespace between the two)"
)

var (
	ListIssueTrackedTimesTool = mcp.NewTool(
		ListIssueTrackedTimesToolName,
		mcp.WithDescription("List tracked time entries on an issue or pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	ListRepoTrackedTimesTool = mcp.NewTool(
		ListRepoTrackedTimesToolName,
		mcp.WithDescription("List tracked time entries across all issues and PRs in a repository"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithString("user", mcp.Description(params.TimeUserFilter)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	ListMyTrackedTimesTool = mcp.NewTool(
		ListMyTrackedTimesToolName,
		mcp.WithDescription("List tracked time entries for the authenticated user across all repositories"),
	)

	AddIssueTimeTool = mcp.NewTool(
		AddIssueTimeToolName,
		mcp.WithDescription("Log time against an issue or pull request. Provide exactly one of 'seconds' or 'duration'."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
		mcp.WithNumber("seconds", mcp.Description(params.TimeSeconds)),
		mcp.WithString("duration", mcp.Description(params.TimeDuration)),
		mcp.WithString("created_at", mcp.Description(params.TimeCreatedAt)),
		mcp.WithString("user_name", mcp.Description(params.TimeUserName)),
	)

	ResetIssueTimeTool = mcp.NewTool(
		ResetIssueTimeToolName,
		mcp.WithDescription("Delete ALL tracked time entries on an issue or pull request (including entries from other users). This is destructive and cannot be undone."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	DeleteIssueTimeEntryTool = mcp.NewTool(
		DeleteIssueTimeEntryToolName,
		mcp.WithDescription("Delete a single tracked time entry by its ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
		mcp.WithNumber("time_id", mcp.Required(), mcp.Description(params.TimeID)),
	)

	StartIssueStopwatchTool = mcp.NewTool(
		StartIssueStopwatchToolName,
		mcp.WithDescription("Start a stopwatch on an issue or pull request. Only one stopwatch per issue; fails if one is already running."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	StopIssueStopwatchTool = mcp.NewTool(
		StopIssueStopwatchToolName,
		mcp.WithDescription("Stop a running stopwatch and record the elapsed time as a tracked time entry"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	CancelIssueStopwatchTool = mcp.NewTool(
		CancelIssueStopwatchToolName,
		mcp.WithDescription("Cancel a running stopwatch without recording a tracked time entry"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	ListMyStopwatchesTool = mcp.NewTool(
		ListMyStopwatchesToolName,
		mcp.WithDescription("List all currently running stopwatches for the authenticated user"),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(ListIssueTrackedTimesTool, ListIssueTrackedTimesFn)
	s.AddTool(ListRepoTrackedTimesTool, ListRepoTrackedTimesFn)
	s.AddTool(ListMyTrackedTimesTool, ListMyTrackedTimesFn)
	s.AddTool(AddIssueTimeTool, AddIssueTimeFn)
	s.AddTool(ResetIssueTimeTool, ResetIssueTimeFn)
	s.AddTool(DeleteIssueTimeEntryTool, DeleteIssueTimeEntryFn)
	s.AddTool(StartIssueStopwatchTool, StartIssueStopwatchFn)
	s.AddTool(StopIssueStopwatchTool, StopIssueStopwatchFn)
	s.AddTool(CancelIssueStopwatchTool, CancelIssueStopwatchFn)
	s.AddTool(ListMyStopwatchesTool, ListMyStopwatchesFn)
}

func listOptions(args map[string]any) (forgejo_sdk.ListOptions, error) {
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 20
	}
	return forgejo_sdk.ListOptions{Page: int(page), PageSize: int(limit)}, nil
}

func parseTimeFilters(args map[string]any) (since, before time.Time, err error) {
	if s, ok := args["since"].(string); ok && s != "" {
		since, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return since, before, fmt.Errorf("invalid since (expected RFC3339): %v", err)
		}
	}
	if b, ok := args["before"].(string); ok && b != "" {
		before, err = time.Parse(time.RFC3339, b)
		if err != nil {
			return since, before, fmt.Errorf("invalid before (expected RFC3339): %v", err)
		}
	}
	return since, before, nil
}

func ListIssueTrackedTimesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueTrackedTimesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])

	lo, err := listOptions(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	since, before, err := parseTimeFilters(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.ListTrackedTimesOptions{
		ListOptions: lo,
		Since:       since,
		Before:      before,
	}
	// user filter is ignored by the issue-level endpoint; see docs/plans/issue-time-tracking.md

	times, _, err := forgejo.Client().ListIssueTrackedTimes(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list issue tracked times err: %v", err))
	}
	return to.TextResult(times)
}

func ListRepoTrackedTimesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoTrackedTimesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	user, _ := args["user"].(string)

	lo, err := listOptions(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	since, before, err := parseTimeFilters(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.ListTrackedTimesOptions{
		ListOptions: lo,
		Since:       since,
		Before:      before,
		User:        user,
	}

	times, _, err := forgejo.Client().ListRepoTrackedTimes(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo tracked times err: %v", err))
	}
	return to.TextResult(times)
}

func ListMyTrackedTimesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListMyTrackedTimesFn")
	times, _, err := forgejo.Client().GetMyTrackedTimes()
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list my tracked times err: %v", err))
	}
	return to.TextResult(times)
}

// resolveSeconds picks the positive integer number of seconds to log from
// either the `seconds` or `duration` argument. Exactly one must be provided.
func resolveSeconds(args map[string]any) (int64, error) {
	secsF, hasSecs := to.Float64Ok(args["seconds"])
	durStr, _ := args["duration"].(string)
	hasDur := durStr != ""

	switch {
	case hasSecs && hasDur:
		return 0, fmt.Errorf("provide exactly one of 'seconds' or 'duration', not both")
	case !hasSecs && !hasDur:
		return 0, fmt.Errorf("one of 'seconds' or 'duration' is required")
	case hasSecs:
		if secsF <= 0 {
			return 0, fmt.Errorf("seconds must be positive, got %v", secsF)
		}
		return int64(secsF), nil
	default:
		d, err := time.ParseDuration(durStr)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %v", durStr, err)
		}
		if d <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %s", d)
		}
		return int64(d / time.Second), nil
	}
}

func AddIssueTimeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddIssueTimeFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])

	seconds, err := resolveSeconds(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.AddTimeOption{Time: seconds}
	if u, ok := args["user_name"].(string); ok && u != "" {
		opt.User = u
	}
	if ts, ok := args["created_at"].(string); ok && ts != "" {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid created_at (expected RFC3339): %v", err))
		}
		opt.Created = parsed
	}

	entry, _, err := forgejo.Client().AddTime(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("add issue time err: %v", err))
	}
	return to.TextResult(entry)
}

func ResetIssueTimeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ResetIssueTimeFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])

	_, err := forgejo.Client().ResetIssueTime(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("reset issue time err: %v", err))
	}
	return to.TextResult("Reset tracked time success")
}

func DeleteIssueTimeEntryFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueTimeEntryFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])
	timeID, _ := to.Float64(args["time_id"])

	_, err := forgejo.Client().DeleteTime(owner, repo, int64(index), int64(timeID))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete time entry err: %v", err))
	}
	return to.TextResult("Delete time entry success")
}

func StartIssueStopwatchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called StartIssueStopwatchFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])

	_, err := forgejo.Client().StartIssueStopWatch(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("start issue stopwatch err: %v", err))
	}
	return to.TextResult("Stopwatch started")
}

func StopIssueStopwatchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called StopIssueStopwatchFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])

	_, err := forgejo.Client().StopIssueStopWatch(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("stop issue stopwatch err: %v", err))
	}
	return to.TextResult("Stopwatch stopped; elapsed time recorded as a tracked time entry")
}

func CancelIssueStopwatchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CancelIssueStopwatchFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])

	_, err := forgejo.Client().DeleteIssueStopwatch(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("cancel issue stopwatch err: %v", err))
	}
	return to.TextResult("Stopwatch cancelled")
}

func ListMyStopwatchesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListMyStopwatchesFn")
	watches, _, err := forgejo.Client().GetMyStopwatches()
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list my stopwatches err: %v", err))
	}
	return to.TextResult(watches)
}
