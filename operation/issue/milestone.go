// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateMilestoneToolName = "create_milestone"
	GetMilestoneToolName    = "get_milestone"
	EditMilestoneToolName   = "edit_milestone"
	DeleteMilestoneToolName = "delete_milestone"
)

var (
	CreateMilestoneTool = mcp.NewTool(
		CreateMilestoneToolName,
		mcp.WithDescription("Create a repository milestone. Returns the created milestone including its numeric id (use with update_issue)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("title", mcp.Required(), mcp.Description("Milestone title")),
		mcp.WithString("description", mcp.Description("Milestone description")),
		mcp.WithString("state", mcp.Description("Milestone state (open|closed)"), mcp.DefaultString("open")),
		mcp.WithString("due_on", mcp.Description("Milestone deadline (RFC3339, e.g. 2026-08-20T00:00:00Z)")),
	)

	GetMilestoneTool = mcp.NewTool(
		GetMilestoneToolName,
		mcp.WithDescription("Get a single repository milestone by ID."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Milestone ID")),
	)

	EditMilestoneTool = mcp.NewTool(
		EditMilestoneToolName,
		mcp.WithDescription("Edit a repository milestone (PATCH — only supplied fields change). Providing no fields is an error."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Milestone ID")),
		mcp.WithString("title", mcp.Description("New milestone title")),
		mcp.WithString("description", mcp.Description("New milestone description")),
		mcp.WithString("state", mcp.Description("New milestone state (open|closed)")),
		mcp.WithString("due_on", mcp.Description("New deadline (RFC3339). The Forgejo API cannot clear an existing deadline, only replace it.")),
	)

	DeleteMilestoneTool = mcp.NewTool(
		DeleteMilestoneToolName,
		mcp.WithDescription("Delete a repository milestone by ID. Issues assigned to it are kept and simply lose the milestone."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Milestone ID")),
	)
)

func RegisterMilestoneTool(s *server.MCPServer) {
	s.AddTool(CreateMilestoneTool, CreateMilestoneFn)
	s.AddTool(GetMilestoneTool, GetMilestoneFn)
	s.AddTool(EditMilestoneTool, EditMilestoneFn)
	s.AddTool(DeleteMilestoneTool, DeleteMilestoneFn)
}

// parseMilestoneState accepts "open" or "closed" (empty means unset) and
// rejects anything else so a typo does not silently reach the API.
func parseMilestoneState(state string) (forgejo_sdk.StateType, error) {
	switch state {
	case "", "open", "closed":
		return forgejo_sdk.StateType(state), nil
	default:
		return "", fmt.Errorf("invalid state %q (expected open or closed)", state)
	}
}

func CreateMilestoneFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateMilestoneFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)
	state, _ := args["state"].(string)
	dueOn, _ := args["due_on"].(string)

	if title == "" {
		return to.ErrorResult(fmt.Errorf("title is required"))
	}
	st, err := parseMilestoneState(state)
	if err != nil {
		return to.ErrorResult(err)
	}
	opt := forgejo_sdk.CreateMilestoneOption{
		Title:       title,
		Description: description,
		State:       st,
	}
	if dueOn != "" {
		parsed, err := time.Parse(time.RFC3339, dueOn)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid due_on format (expected RFC3339): %w", err))
		}
		opt.Deadline = &parsed
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	milestone, _, err := client.CreateMilestone(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create milestone err: %w", err))
	}
	return to.TextResult(milestone)
}

func GetMilestoneFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetMilestoneFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	id, _ := to.Float64(args["id"])

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	milestone, _, err := client.GetMilestone(owner, repo, int64(id))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get milestone err: %w", err))
	}
	return to.TextResult(milestone)
}

func EditMilestoneFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditMilestoneFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	id, _ := to.Float64(args["id"])
	title, _ := args["title"].(string)
	description, hasDescription := args["description"].(string)
	state, _ := args["state"].(string)
	dueOn, _ := args["due_on"].(string)

	if title == "" && !hasDescription && state == "" && dueOn == "" {
		return to.ErrorResult(fmt.Errorf("at least one of title, description, state, or due_on must be provided"))
	}

	opt := forgejo_sdk.EditMilestoneOption{Title: title}
	if hasDescription {
		opt.Description = &description
	}
	if state != "" {
		st, err := parseMilestoneState(state)
		if err != nil {
			return to.ErrorResult(err)
		}
		opt.State = &st
	}
	if dueOn != "" {
		parsed, err := time.Parse(time.RFC3339, dueOn)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid due_on format (expected RFC3339): %w", err))
		}
		opt.Deadline = &parsed
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	milestone, _, err := client.EditMilestone(owner, repo, int64(id), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit milestone err: %w", err))
	}
	return to.TextResult(milestone)
}

func DeleteMilestoneFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteMilestoneFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	id, _ := to.Float64(args["id"])

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	_, err = client.DeleteMilestone(owner, repo, int64(id))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete milestone err: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Deleted milestone %d", int64(id)))
}
