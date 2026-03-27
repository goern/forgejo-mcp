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
)

const (
	ListOrgMembersToolName     = "list_org_members"
	CheckOrgMembershipToolName = "check_org_membership"
	RemoveOrgMemberToolName    = "remove_org_member"
)

var (
	ListOrgMembersTool = mcp.NewTool(
		ListOrgMembersToolName,
		mcp.WithDescription("List members of an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	CheckOrgMembershipTool = mcp.NewTool(
		CheckOrgMembershipToolName,
		mcp.WithDescription("Check if a user is a member of an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)

	RemoveOrgMemberTool = mcp.NewTool(
		RemoveOrgMemberToolName,
		mcp.WithDescription("Remove a member from an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)
)

func ListOrgMembersFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListOrgMembersFn")
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

	opt := forgejo_sdk.ListOrgMembershipOption{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	members, _, err := forgejo.Client().ListOrgMembership(orgName, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list org members err: %v", err))
	}
	return to.TextResult(members)
}

func CheckOrgMembershipFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CheckOrgMembershipFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}
	user, ok := req.GetArguments()["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("username is required"))
	}

	isMember, _, err := forgejo.Client().CheckOrgMembership(orgName, user)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("check org membership err: %v", err))
	}
	return to.TextResult(map[string]interface{}{
		"org":       orgName,
		"user":      user,
		"is_member": isMember,
	})
}

func RemoveOrgMemberFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveOrgMemberFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}
	user, ok := req.GetArguments()["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("username is required"))
	}

	_, err := forgejo.Client().DeleteOrgMembership(orgName, user)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("remove org member err: %v", err))
	}
	return to.TextResult(fmt.Sprintf("User '%s' removed from organization '%s'", user, orgName))
}
