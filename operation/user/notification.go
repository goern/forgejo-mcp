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
	CheckNotificationsToolName = "check_notifications"
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
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			opt.Since = t
		}
	}

	if beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			opt.Before = t
		}
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
