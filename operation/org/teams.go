package org

import (
	"context"
	"errors"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListOrgTeamsToolName    = "list_org_teams"
	CreateOrgTeamToolName   = "create_org_team"
	AddTeamMemberToolName   = "add_team_member"
	RemoveTeamMemberToolName = "remove_team_member"
	AddTeamRepoToolName     = "add_team_repo"
	RemoveTeamRepoToolName  = "remove_team_repo"
)

var (
	ListOrgTeamsTool = mcp.NewTool(
		ListOrgTeamsToolName,
		mcp.WithDescription("List teams in an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	CreateOrgTeamTool = mcp.NewTool(
		CreateOrgTeamToolName,
		mcp.WithDescription("Create a team in an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Team name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("permission", mcp.Description("Access level: read, write, or admin (default: read)")),
		mcp.WithBoolean("can_create_org_repo", mcp.Description("Whether members can create repos in the org")),
		mcp.WithBoolean("includes_all_repositories", mcp.Description("Whether team has access to all org repos")),
	)

	AddTeamMemberTool = mcp.NewTool(
		AddTeamMemberToolName,
		mcp.WithDescription("Add a user to a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)

	RemoveTeamMemberTool = mcp.NewTool(
		RemoveTeamMemberToolName,
		mcp.WithDescription("Remove a user from a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)

	AddTeamRepoTool = mcp.NewTool(
		AddTeamRepoToolName,
		mcp.WithDescription("Add a repository to a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)

	RemoveTeamRepoTool = mcp.NewTool(
		RemoveTeamRepoToolName,
		mcp.WithDescription("Remove a repository from a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)
)

func ListOrgTeamsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListOrgTeamsFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 100
	}

	opt := forgejo_sdk.ListTeamsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	teams, _, err := forgejo.Client().ListOrgTeams(orgName, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list org teams err: %v", err))
	}
	return to.TextResult(teams)
}

func CreateOrgTeamFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrgTeamFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}
	name, ok := req.GetArguments()["name"].(string)
	if !ok {
		return to.ErrorResult(errors.New("team name is required"))
	}

	permission := forgejo_sdk.AccessModeRead
	if v, ok := req.GetArguments()["permission"].(string); ok && v != "" {
		permission = forgejo_sdk.AccessMode(v)
	}

	opt := forgejo_sdk.CreateTeamOption{
		Name:       name,
		Permission: permission,
	}
	if v, ok := req.GetArguments()["description"].(string); ok {
		opt.Description = v
	}
	if v, ok := req.GetArguments()["can_create_org_repo"].(bool); ok {
		opt.CanCreateOrgRepo = v
	}
	if v, ok := req.GetArguments()["includes_all_repositories"].(bool); ok {
		opt.IncludesAllRepositories = v
	}

	team, _, err := forgejo.Client().CreateTeam(orgName, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create org team err: %v", err))
	}
	return to.TextResult(team)
}

func AddTeamMemberFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddTeamMemberFn")
	teamIDFloat, ok := req.GetArguments()["team_id"].(float64)
	if !ok {
		return to.ErrorResult(errors.New("team_id is required"))
	}
	teamID := int64(teamIDFloat)
	user, ok := req.GetArguments()["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("username is required"))
	}

	_, err := forgejo.Client().AddTeamMember(teamID, user)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("add team member err: %v", err))
	}
	return to.TextResult(fmt.Sprintf("User '%s' added to team %d", user, teamID))
}

func RemoveTeamMemberFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveTeamMemberFn")
	teamIDFloat, ok := req.GetArguments()["team_id"].(float64)
	if !ok {
		return to.ErrorResult(errors.New("team_id is required"))
	}
	teamID := int64(teamIDFloat)
	user, ok := req.GetArguments()["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("username is required"))
	}

	_, err := forgejo.Client().RemoveTeamMember(teamID, user)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("remove team member err: %v", err))
	}
	return to.TextResult(fmt.Sprintf("User '%s' removed from team %d", user, teamID))
}

func AddTeamRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddTeamRepoFn")
	teamIDFloat, ok := req.GetArguments()["team_id"].(float64)
	if !ok {
		return to.ErrorResult(errors.New("team_id is required"))
	}
	teamID := int64(teamIDFloat)
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(errors.New("repository name is required"))
	}

	_, err := forgejo.Client().AddTeamRepository(teamID, orgName, repo)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("add team repo err: %v", err))
	}
	return to.TextResult(fmt.Sprintf("Repository '%s/%s' added to team %d", orgName, repo, teamID))
}

func RemoveTeamRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveTeamRepoFn")
	teamIDFloat, ok := req.GetArguments()["team_id"].(float64)
	if !ok {
		return to.ErrorResult(errors.New("team_id is required"))
	}
	teamID := int64(teamIDFloat)
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(errors.New("repository name is required"))
	}

	_, err := forgejo.Client().RemoveTeamRepository(teamID, orgName, repo)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("remove team repo err: %v", err))
	}
	return to.TextResult(fmt.Sprintf("Repository '%s/%s' removed from team %d", orgName, repo, teamID))
}
