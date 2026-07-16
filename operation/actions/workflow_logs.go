// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListActionRunJobsToolName = "list_action_run_jobs"
	GetActionJobLogsToolName  = "get_action_job_logs"

	defaultActionJobsLimit = 30
	maxActionJobsLimit     = 100
	defaultActionLogBytes  = 32 * 1024
	maxActionLogBytes      = 256 * 1024
)

type actionRunJob struct {
	ID      int64    `json:"id"`
	RunID   int64    `json:"run_id"`
	Attempt int64    `json:"attempt"`
	Handle  string   `json:"handle"`
	RepoID  int64    `json:"repo_id"`
	OwnerID int64    `json:"owner_id"`
	Name    string   `json:"name"`
	Needs   []string `json:"needs"`
	RunsOn  []string `json:"runs_on"`
	TaskID  int64    `json:"task_id"`
	Status  string   `json:"status"`
}

type actionRunJobsResult struct {
	RunID      int64          `json:"run_id"`
	Jobs       []actionRunJob `json:"jobs"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalCount int            `json:"total_count"`
	HasNext    bool           `json:"has_next"`
	NextPage   int            `json:"next_page,omitempty"`
}

type actionJobLogResult struct {
	JobID            int64  `json:"job_id"`
	Attempt          int64  `json:"attempt,omitempty"`
	Content          string `json:"content"`
	ContentType      string `json:"content_type"`
	ContentRange     string `json:"content_range"`
	StartByte        int64  `json:"start_byte"`
	EndByte          int64  `json:"end_byte"`
	TotalBytes       int64  `json:"total_bytes"`
	BytesReturned    int    `json:"bytes_returned"`
	TruncatedBefore  bool   `json:"truncated_before"`
	TruncatedAfter   bool   `json:"truncated_after"`
	PreviousOffset   *int64 `json:"previous_offset,omitempty"`
	PreviousMaxBytes *int   `json:"previous_max_bytes,omitempty"`
	NextOffset       *int64 `json:"next_offset,omitempty"`
}

var (
	ListActionRunJobsTool = mcp.NewTool(
		ListActionRunJobsToolName,
		mcp.WithDescription("List jobs for a Forgejo v16+ workflow run. Results are paged client-side because Forgejo returns the full job list."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionJobsLimit), mcp.Min(1), mcp.Max(maxActionJobsLimit)),
	)

	GetActionJobLogsTool = mcp.NewTool(
		GetActionJobLogsToolName,
		mcp.WithDescription("Read a bounded byte range from a Forgejo v16+ workflow job's plaintext log. Omitting offset returns the tail. Continue backward with previous_offset and previous_max_bytes, or forward with next_offset."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("job_id", mcp.Required(), mcp.Description(params.JobID), mcp.Min(1)),
		mcp.WithNumber("attempt", mcp.Description(params.Attempt), mcp.Min(1)),
		mcp.WithNumber("offset", mcp.Description(params.LogOffset), mcp.Min(0)),
		mcp.WithNumber("max_bytes", mcp.Description(params.LogMaxBytes), mcp.DefaultNumber(defaultActionLogBytes), mcp.Min(1), mcp.Max(maxActionLogBytes)),
	)
)

func ListActionRunJobsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListActionRunJobsFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(args, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntegerArg(args, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntegerArg(args, "limit", defaultActionJobsLimit, 1, maxActionJobsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, repo, runID)
	allJobs := make([]actionRunJob, 0)
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &allJobs); err != nil {
		return to.ErrorResult(fmt.Errorf("list action run jobs: %w", err))
	}

	start64 := int64(page-1) * int64(limit)
	start := len(allJobs)
	if start64 < int64(len(allJobs)) {
		start = int(start64)
	}
	end := min(start+limit, len(allJobs))
	jobs := allJobs[start:end]
	hasNext := end < len(allJobs)
	result := actionRunJobsResult{
		RunID:      runID,
		Jobs:       jobs,
		Page:       page,
		Limit:      limit,
		TotalCount: len(allJobs),
		HasNext:    hasNext,
	}
	if hasNext {
		result.NextPage = page + 1
	}

	return to.SafeTextResult(result)
}

func GetActionJobLogsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetActionJobLogsFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	jobID, err := positiveIntegerArg(args, "job_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}
	attempt, err := positiveIntegerArg(args, "attempt", false)
	if err != nil {
		return to.ErrorResult(err)
	}
	maxBytes, err := boundedIntegerArg(args, "max_bytes", defaultActionLogBytes, 1, maxActionLogBytes)
	if err != nil {
		return to.ErrorResult(err)
	}

	offset, hasOffset, err := optionalNonNegativeIntegerArg(args, "offset")
	if err != nil {
		return to.ErrorResult(err)
	}
	byteRange := fmt.Sprintf("bytes=-%d", maxBytes)
	if hasOffset {
		if offset > math.MaxInt64-int64(maxBytes) {
			return to.ErrorResult(errors.New("offset and max_bytes exceed the supported integer range"))
		}
		byteRange = fmt.Sprintf("bytes=%d-%d", offset, offset+int64(maxBytes)-1)
	}

	path := fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", owner, repo, jobID)
	if attempt > 0 {
		query := url.Values{}
		query.Set("attempt", fmt.Sprintf("%d", attempt))
		path += "?" + query.Encode()
	}
	resp, err := forgejo.DoAPIRaw(ctx, path, "text/plain", byteRange, int64(maxBytes))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get action job logs: %w", err))
	}

	startByte, endByte, totalBytes, err := responseByteRange(resp)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get action job logs: %w", err))
	}
	result := actionJobLogResult{
		JobID:           jobID,
		Attempt:         attempt,
		Content:         strings.ToValidUTF8(string(resp.Body), "\ufffd"),
		ContentType:     resp.ContentType,
		ContentRange:    resp.ContentRange,
		StartByte:       startByte,
		EndByte:         endByte,
		TotalBytes:      totalBytes,
		BytesReturned:   len(resp.Body),
		TruncatedBefore: startByte > 0,
		TruncatedAfter:  totalBytes > 0 && endByte+1 < totalBytes,
	}
	if result.TruncatedBefore {
		previousBytes := min(int64(maxBytes), startByte)
		previous := startByte - previousBytes
		previousMaxBytes := int(previousBytes)
		result.PreviousOffset = &previous
		result.PreviousMaxBytes = &previousMaxBytes
	}
	if result.TruncatedAfter {
		next := endByte + 1
		result.NextOffset = &next
	}

	// Do not use TextResult here: it debug-logs the complete payload, and CI
	// logs may contain sensitive output even when Forgejo masks known secrets.
	return to.SafeTextResult(result)
}

func requiredRepo(args map[string]any) (string, string, error) {
	owner, ok := args["owner"].(string)
	if !ok || owner == "" {
		return "", "", errors.New("owner is required")
	}
	repo, ok := args["repo"].(string)
	if !ok || repo == "" {
		return "", "", errors.New("repo is required")
	}
	return owner, repo, nil
}

func positiveIntegerArg(args map[string]any, name string, required bool) (int64, error) {
	value, exists := args[name]
	if !exists || value == nil || value == "" {
		if required {
			return 0, fmt.Errorf("%s is required", name)
		}
		return 0, nil
	}
	number, err := to.Float64(value)
	if err != nil || number <= 0 || math.Trunc(number) != number || number >= float64(math.MaxInt64) {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int64(number), nil
}

func optionalNonNegativeIntegerArg(args map[string]any, name string) (int64, bool, error) {
	value, exists := args[name]
	if !exists || value == nil || value == "" {
		return 0, false, nil
	}
	number, err := to.Float64(value)
	if err != nil || number < 0 || math.Trunc(number) != number || number >= float64(math.MaxInt64) {
		return 0, false, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return int64(number), true, nil
}

func boundedIntegerArg(args map[string]any, name string, defaultValue, minimum, maximum int) (int, error) {
	value, exists := args[name]
	if !exists || value == nil || value == "" {
		return defaultValue, nil
	}
	number, err := to.Float64(value)
	if err != nil || math.Trunc(number) != number || number < float64(minimum) || number > float64(maximum) {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", name, minimum, maximum)
	}
	return int(number), nil
}

func responseByteRange(resp *forgejo.RawAPIResponse) (int64, int64, int64, error) {
	if resp.StatusCode == http.StatusPartialContent {
		var start, end, total int64
		if _, err := fmt.Sscanf(resp.ContentRange, "bytes %d-%d/%d", &start, &end, &total); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid Content-Range %q: %w", resp.ContentRange, err)
		}
		if start < 0 || end < start || total < end+1 || int64(len(resp.Body)) != end-start+1 {
			return 0, 0, 0, fmt.Errorf("inconsistent Content-Range %q for %d response bytes", resp.ContentRange, len(resp.Body))
		}
		return start, end, total, nil
	}

	total := resp.ContentLength
	if total < 0 {
		total = int64(len(resp.Body))
	}
	if total != int64(len(resp.Body)) {
		return 0, 0, 0, fmt.Errorf("response declared %d bytes but returned %d", total, len(resp.Body))
	}
	if total == 0 {
		return 0, -1, 0, nil
	}
	return 0, total - 1, total, nil
}
