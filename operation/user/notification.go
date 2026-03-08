package user

import (
	"context"
	"fmt"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	CheckNotificationsToolName        = "check_notifications"
	GetNotificationThreadToolName     = "get_notification_thread"
	MarkNotificationReadToolName      = "mark_notification_read"
	MarkAllNotificationsReadToolName  = "mark_all_notifications_read"
	ListRepoNotificationsToolName     = "list_repo_notifications"
	MarkRepoNotificationsReadToolName = "mark_repo_notifications_read"
)

var (
	CheckNotificationsTool = mcp.NewTool(
		CheckNotificationsToolName,
		mcp.WithDescription("Check and list user notifications"),
		mcp.WithBoolean("all", mcp.Description("Include read notifications (default: false)")),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetNotificationThreadTool = mcp.NewTool(
		GetNotificationThreadToolName,
		mcp.WithDescription("Get detailed info on a single notification thread"),
		mcp.WithNumber("id", mcp.Description("Notification ID"), mcp.Required()),
	)

	MarkNotificationReadTool = mcp.NewTool(
		MarkNotificationReadToolName,
		mcp.WithDescription("Mark a single notification thread as read"),
		mcp.WithNumber("id", mcp.Description("Notification ID"), mcp.Required()),
	)

	MarkAllNotificationsReadTool = mcp.NewTool(
		MarkAllNotificationsReadToolName,
		mcp.WithDescription("Acknowledge all notifications"),
		mcp.WithString("last_read_at", mcp.Description("Optional RFC3339 time")),
	)

	ListRepoNotificationsTool = mcp.NewTool(
		ListRepoNotificationsToolName,
		mcp.WithDescription("Filter notifications scoped to a single repository"),
		mcp.WithString("owner", mcp.Description("Repository owner"), mcp.Required()),
		mcp.WithString("repo", mcp.Description("Repository name"), mcp.Required()),
		mcp.WithBoolean("all", mcp.Description("Include read notifications (default: false)")),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	MarkRepoNotificationsReadTool = mcp.NewTool(
		MarkRepoNotificationsReadToolName,
		mcp.WithDescription("Mark all notifications in a specific repo as read"),
		mcp.WithString("owner", mcp.Description("Repository owner"), mcp.Required()),
		mcp.WithString("repo", mcp.Description("Repository name"), mcp.Required()),
		mcp.WithString("last_read_at", mcp.Description("Optional RFC3339 time")),
	)
)

func CheckNotificationsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, _ = log.WithMCPContext(ctx, CheckNotificationsToolName)
	start := time.Now()

	log.LogMCPToolStart(ctx, CheckNotificationsToolName, req.GetArguments())

	// Parse arguments
	all, ok := req.GetArguments()["all"].(bool)
	if !ok {
		all = false
	}
	sinceStr, _ := req.GetArguments()["since"].(string)
	beforeStr, _ := req.GetArguments()["before"].(string)
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 20
	}

	opt := forgejo_sdk.ListNotificationOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	if !all {
		opt.Status = []forgejo_sdk.NotifyStatus{forgejo_sdk.NotifyStatusUnread}
	}

	if sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			log.LogMCPToolError(ctx, CheckNotificationsToolName, time.Since(start), err)
			return to.ErrorResult(fmt.Errorf("invalid since time format (expected RFC3339): %v", err))
		}
		opt.Since = t
	}

	if beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			log.LogMCPToolError(ctx, CheckNotificationsToolName, time.Since(start), err)
			return to.ErrorResult(fmt.Errorf("invalid before time format (expected RFC3339): %v", err))
		}
		opt.Before = t
	}

	notifications, resp, err := forgejo.Client().ListNotifications(opt)
	duration := time.Since(start)

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	forgejo.LogAPICall(ctx, "GET", "/notifications", duration, statusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, CheckNotificationsToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("list notifications err: %v", err))
	}

	log.LogMCPToolComplete(ctx, CheckNotificationsToolName, duration, fmt.Sprintf("Retrieved %d notifications", len(notifications)))

	return to.TextResult(notifications)
}

func GetNotificationThreadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, _ = log.WithMCPContext(ctx, GetNotificationThreadToolName)
	start := time.Now()
	log.LogMCPToolStart(ctx, GetNotificationThreadToolName, req.GetArguments())

	id, ok := req.GetArguments()["id"].(float64)
	if !ok {
		err := fmt.Errorf("id is required and must be a number")
		log.LogMCPToolError(ctx, GetNotificationThreadToolName, time.Since(start), err)
		return to.ErrorResult(err)
	}

	thread, resp, err := forgejo.Client().GetNotification(int64(id))
	duration := time.Since(start)

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	forgejo.LogAPICall(ctx, "GET", fmt.Sprintf("/notifications/threads/%d", int64(id)), duration, statusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, GetNotificationThreadToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("get notification thread err: %v", err))
	}

	log.LogMCPToolComplete(ctx, GetNotificationThreadToolName, duration, "Retrieved notification thread")
	return to.TextResult(thread)
}

func MarkNotificationReadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, _ = log.WithMCPContext(ctx, MarkNotificationReadToolName)
	start := time.Now()
	log.LogMCPToolStart(ctx, MarkNotificationReadToolName, req.GetArguments())

	id, ok := req.GetArguments()["id"].(float64)
	if !ok {
		err := fmt.Errorf("id is required and must be a number")
		log.LogMCPToolError(ctx, MarkNotificationReadToolName, time.Since(start), err)
		return to.ErrorResult(err)
	}

	thread, resp, err := forgejo.Client().ReadNotification(int64(id))
	duration := time.Since(start)

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	forgejo.LogAPICall(ctx, "PATCH", fmt.Sprintf("/notifications/threads/%d", int64(id)), duration, statusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, MarkNotificationReadToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("mark notification read err: %v", err))
	}

	log.LogMCPToolComplete(ctx, MarkNotificationReadToolName, duration, "Marked notification as read")
	return to.TextResult(thread)
}

func MarkAllNotificationsReadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, _ = log.WithMCPContext(ctx, MarkAllNotificationsReadToolName)
	start := time.Now()
	log.LogMCPToolStart(ctx, MarkAllNotificationsReadToolName, req.GetArguments())

	opt := forgejo_sdk.MarkNotificationOptions{}
	if lastReadAtStr, ok := req.GetArguments()["last_read_at"].(string); ok && lastReadAtStr != "" {
		t, err := time.Parse(time.RFC3339, lastReadAtStr)
		if err != nil {
			log.LogMCPToolError(ctx, MarkAllNotificationsReadToolName, time.Since(start), err)
			return to.ErrorResult(fmt.Errorf("invalid last_read_at time format (expected RFC3339): %v", err))
		}
		opt.LastReadAt = t
	}

	_, resp, err := forgejo.Client().ReadNotifications(opt)
	duration := time.Since(start)

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	forgejo.LogAPICall(ctx, "PUT", "/notifications", duration, statusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, MarkAllNotificationsReadToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("mark all notifications read err: %v", err))
	}

	log.LogMCPToolComplete(ctx, MarkAllNotificationsReadToolName, duration, "Marked all notifications as read")
	return to.TextResult("All notifications marked as read")
}

func ListRepoNotificationsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, _ = log.WithMCPContext(ctx, ListRepoNotificationsToolName)
	start := time.Now()
	log.LogMCPToolStart(ctx, ListRepoNotificationsToolName, req.GetArguments())

	owner, ok1 := req.GetArguments()["owner"].(string)
	repo, ok2 := req.GetArguments()["repo"].(string)
	if !ok1 || !ok2 {
		err := fmt.Errorf("owner and repo are required")
		log.LogMCPToolError(ctx, ListRepoNotificationsToolName, time.Since(start), err)
		return to.ErrorResult(err)
	}

	all, ok := req.GetArguments()["all"].(bool)
	if !ok {
		all = false
	}
	sinceStr, _ := req.GetArguments()["since"].(string)
	beforeStr, _ := req.GetArguments()["before"].(string)
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 20
	}

	opt := forgejo_sdk.ListNotificationOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	if !all {
		opt.Status = []forgejo_sdk.NotifyStatus{forgejo_sdk.NotifyStatusUnread}
	}

	if sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			log.LogMCPToolError(ctx, ListRepoNotificationsToolName, time.Since(start), err)
			return to.ErrorResult(fmt.Errorf("invalid since time format (expected RFC3339): %v", err))
		}
		opt.Since = t
	}

	if beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			log.LogMCPToolError(ctx, ListRepoNotificationsToolName, time.Since(start), err)
			return to.ErrorResult(fmt.Errorf("invalid before time format (expected RFC3339): %v", err))
		}
		opt.Before = t
	}

	notifications, resp, err := forgejo.Client().ListRepoNotifications(owner, repo, opt)
	duration := time.Since(start)

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	forgejo.LogAPICall(ctx, "GET", fmt.Sprintf("/repos/%s/%s/notifications", owner, repo), duration, statusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, ListRepoNotificationsToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("list repo notifications err: %v", err))
	}

	log.LogMCPToolComplete(ctx, ListRepoNotificationsToolName, duration, fmt.Sprintf("Retrieved %d notifications for repo %s/%s", len(notifications), owner, repo))
	return to.TextResult(notifications)
}

func MarkRepoNotificationsReadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, _ = log.WithMCPContext(ctx, MarkRepoNotificationsReadToolName)
	start := time.Now()
	log.LogMCPToolStart(ctx, MarkRepoNotificationsReadToolName, req.GetArguments())

	owner, ok1 := req.GetArguments()["owner"].(string)
	repo, ok2 := req.GetArguments()["repo"].(string)
	if !ok1 || !ok2 {
		err := fmt.Errorf("owner and repo are required")
		log.LogMCPToolError(ctx, MarkRepoNotificationsReadToolName, time.Since(start), err)
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.MarkNotificationOptions{}
	if lastReadAtStr, ok := req.GetArguments()["last_read_at"].(string); ok && lastReadAtStr != "" {
		t, err := time.Parse(time.RFC3339, lastReadAtStr)
		if err != nil {
			log.LogMCPToolError(ctx, MarkRepoNotificationsReadToolName, time.Since(start), err)
			return to.ErrorResult(fmt.Errorf("invalid last_read_at time format (expected RFC3339): %v", err))
		}
		opt.LastReadAt = t
	}

	_, resp, err := forgejo.Client().ReadRepoNotifications(owner, repo, opt)
	duration := time.Since(start)

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	forgejo.LogAPICall(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/notifications", owner, repo), duration, statusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, MarkRepoNotificationsReadToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("mark repo notifications read err: %v", err))
	}

	log.LogMCPToolComplete(ctx, MarkRepoNotificationsReadToolName, duration, fmt.Sprintf("Marked all notifications for repo %s/%s as read", owner, repo))
	return to.TextResult("Repo notifications marked as read")
}
