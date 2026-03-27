package org

import (
	"context"
	"errors"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateOrgToolName   = "create_org"
	GetOrgToolName      = "get_org"
	ListMyOrgsToolName  = "list_my_orgs"
	ListUserOrgsToolName = "list_user_orgs"
	EditOrgToolName     = "edit_org"
	DeleteOrgToolName   = "delete_org"
)

var (
	CreateOrgTool = mcp.NewTool(
		CreateOrgToolName,
		mcp.WithDescription("Create an organization"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Organization username")),
		mcp.WithString("full_name", mcp.Description("Display name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("website", mcp.Description("Website URL")),
		mcp.WithString("location", mcp.Description("Location")),
		mcp.WithString("visibility", mcp.Description("Visibility: public, limited, or private")),
	)

	GetOrgTool = mcp.NewTool(
		GetOrgToolName,
		mcp.WithDescription("Get organization details"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
	)

	ListMyOrgsTool = mcp.NewTool(
		ListMyOrgsToolName,
		mcp.WithDescription("List my organizations"),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	ListUserOrgsTool = mcp.NewTool(
		ListUserOrgsToolName,
		mcp.WithDescription("List a user's organizations"),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	EditOrgTool = mcp.NewTool(
		EditOrgToolName,
		mcp.WithDescription("Edit organization settings"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("full_name", mcp.Description("Display name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("website", mcp.Description("Website URL")),
		mcp.WithString("location", mcp.Description("Location")),
		mcp.WithString("visibility", mcp.Description("Visibility: public, limited, or private")),
	)

	DeleteOrgTool = mcp.NewTool(
		DeleteOrgToolName,
		mcp.WithDescription("Delete an organization. WARNING: This is destructive and irreversible — all repos, teams, and data will be permanently removed"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(CreateOrgTool, CreateOrgFn)
	s.AddTool(GetOrgTool, GetOrgFn)
	s.AddTool(ListMyOrgsTool, ListMyOrgsFn)
	s.AddTool(ListUserOrgsTool, ListUserOrgsFn)
	s.AddTool(EditOrgTool, EditOrgFn)
	s.AddTool(DeleteOrgTool, DeleteOrgFn)

	// Membership
	s.AddTool(ListOrgMembersTool, ListOrgMembersFn)
	s.AddTool(CheckOrgMembershipTool, CheckOrgMembershipFn)
	s.AddTool(RemoveOrgMemberTool, RemoveOrgMemberFn)

	// Teams
	s.AddTool(ListOrgTeamsTool, ListOrgTeamsFn)
	s.AddTool(CreateOrgTeamTool, CreateOrgTeamFn)
	s.AddTool(AddTeamMemberTool, AddTeamMemberFn)
	s.AddTool(RemoveTeamMemberTool, RemoveTeamMemberFn)
	s.AddTool(AddTeamRepoTool, AddTeamRepoFn)
	s.AddTool(RemoveTeamRepoTool, RemoveTeamRepoFn)
}

func CreateOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrgFn")
	name, ok := req.GetArguments()["name"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	opt := forgejo_sdk.CreateOrgOption{
		Name: name,
	}
	if v, ok := req.GetArguments()["full_name"].(string); ok {
		opt.FullName = v
	}
	if v, ok := req.GetArguments()["description"].(string); ok {
		opt.Description = v
	}
	if v, ok := req.GetArguments()["website"].(string); ok {
		opt.Website = v
	}
	if v, ok := req.GetArguments()["location"].(string); ok {
		opt.Location = v
	}
	if v, ok := req.GetArguments()["visibility"].(string); ok {
		opt.Visibility = forgejo_sdk.VisibleType(v)
	}

	org, _, err := forgejo.Client().CreateOrg(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create org err: %v", err))
	}
	return to.TextResult(org)
}

func GetOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetOrgFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	org, _, err := forgejo.Client().GetOrg(orgName)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get org err: %v", err))
	}
	return to.TextResult(org)
}

func ListMyOrgsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListMyOrgsFn")
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 100
	}

	opt := forgejo_sdk.ListOrgsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	orgs, _, err := forgejo.Client().ListMyOrgs(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list my orgs err: %v", err))
	}
	return to.TextResult(orgs)
}

func ListUserOrgsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListUserOrgsFn")
	user, ok := req.GetArguments()["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("username is required"))
	}
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 100
	}

	opt := forgejo_sdk.ListOrgsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	orgs, _, err := forgejo.Client().ListUserOrgs(user, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list user orgs err: %v", err))
	}
	return to.TextResult(orgs)
}

func EditOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditOrgFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	opt := forgejo_sdk.EditOrgOption{}
	if v, ok := req.GetArguments()["full_name"].(string); ok {
		opt.FullName = v
	}
	if v, ok := req.GetArguments()["description"].(string); ok {
		opt.Description = v
	}
	if v, ok := req.GetArguments()["website"].(string); ok {
		opt.Website = v
	}
	if v, ok := req.GetArguments()["location"].(string); ok {
		opt.Location = v
	}
	if v, ok := req.GetArguments()["visibility"].(string); ok {
		opt.Visibility = forgejo_sdk.VisibleType(v)
	}

	_, err := forgejo.Client().EditOrg(orgName, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit org err: %v", err))
	}

	// Fetch updated org to return
	org, _, err := forgejo.Client().GetOrg(orgName)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated org err: %v", err))
	}
	return to.TextResult(org)
}

func DeleteOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteOrgFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	_, err := forgejo.Client().DeleteOrg(orgName)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete org err: %v", err))
	}
	return to.TextResult(fmt.Sprintf("Organization '%s' deleted successfully", orgName))
}
