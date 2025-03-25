package search

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/ptr"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	gitea_sdk "code.gitea.io/sdk/gitea"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	SearchUsersToolName    = "search_users"
	SearchOrgTeamsToolName = "search_org_teams"
	SearchReposToolName    = "search_repos"
)

var (
	SearchUsersTool = mcp.NewTool(
		SearchUsersToolName,
		mcp.WithDescription("search users"),
		mcp.WithString("keyword", mcp.Description("Keyword")),
		mcp.WithNumber("page", mcp.Description("Page"), mcp.DefaultNumber(1)),
		mcp.WithNumber("pageSize", mcp.Description("PageSize"), mcp.DefaultNumber(100)),
	)

	SearOrgTeamsTool = mcp.NewTool(
		SearchOrgTeamsToolName,
		mcp.WithDescription("search organization teams"),
		mcp.WithString("org", mcp.Description("organization name")),
		mcp.WithString("query", mcp.Description("search organization teams")),
		mcp.WithBoolean("includeDescription", mcp.Description("include description?")),
		mcp.WithNumber("page", mcp.Description("Page"), mcp.DefaultNumber(1)),
		mcp.WithNumber("pageSize", mcp.Description("PageSize"), mcp.DefaultNumber(100)),
	)

	SearchReposTool = mcp.NewTool(
		SearchReposToolName,
		mcp.WithDescription("search repos"),
		mcp.WithString("keyword", mcp.Description("Keyword")),
		mcp.WithBoolean("keywordIsTopic", mcp.Description("KeywordIsTopic")),
		mcp.WithBoolean("keywordInDescription", mcp.Description("KeywordInDescription")),
		mcp.WithNumber("ownerID", mcp.Description("OwnerID")),
		mcp.WithBoolean("isPrivate", mcp.Description("IsPrivate")),
		mcp.WithBoolean("isArchived", mcp.Description("IsArchived")),
		mcp.WithString("sort", mcp.Description("Sort")),
		mcp.WithString("order", mcp.Description("Order")),
		mcp.WithNumber("page", mcp.Description("Page"), mcp.DefaultNumber(1)),
		mcp.WithNumber("pageSize", mcp.Description("PageSize"), mcp.DefaultNumber(100)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(SearchUsersTool, SearchUsersFn)
	s.AddTool(SearOrgTeamsTool, SearchOrgTeamsFn)
	s.AddTool(SearchReposTool, SearchReposFn)
}

func SearchUsersFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchUsersFn")
	keyword, ok := req.Params.Arguments["keyword"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("keyword is required"))
	}
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	pageSize, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		pageSize = 100
	}
	opt := gitea_sdk.SearchUsersOption{
		KeyWord: keyword,
		ListOptions: gitea_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(pageSize),
		},
	}
	users, _, err := gitea.Client().SearchUsers(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search users err: %v", err))
	}
	return to.TextResult(users)
}

func SearchOrgTeamsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchOrgTeamsFn")
	org, ok := req.Params.Arguments["org"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("organization is required"))
	}
	query, ok := req.Params.Arguments["query"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("query is required"))
	}
	includeDescription, _ := req.Params.Arguments["includeDescription"].(bool)
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	pageSize, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		pageSize = 100
	}
	opt := gitea_sdk.SearchTeamsOptions{
		Query:              query,
		IncludeDescription: includeDescription,
		ListOptions: gitea_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(pageSize),
		},
	}
	teams, _, err := gitea.Client().SearchOrgTeams(org, &opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search organization teams error: %v", err))
	}
	return to.TextResult(teams)
}

func SearchReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchReposFn")
	keyword, ok := req.Params.Arguments["keyword"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("keyword is required"))
	}
	keywordIsTopic, _ := req.Params.Arguments["keywordIsTopic"].(bool)
	keywordInDescription, _ := req.Params.Arguments["keywordInDescription"].(bool)
	ownerID, _ := req.Params.Arguments["ownerID"].(float64)
	isPrivate, _ := req.Params.Arguments["isPrivate"].(bool)
	isArchived, _ := req.Params.Arguments["isArchived"].(bool)
	sort, _ := req.Params.Arguments["sort"].(string)
	order, _ := req.Params.Arguments["order"].(string)
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	pageSize, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		pageSize = 100
	}
	opt := gitea_sdk.SearchRepoOptions{
		Keyword:              keyword,
		KeywordIsTopic:       keywordIsTopic,
		KeywordInDescription: keywordInDescription,
		OwnerID:              int64(ownerID),
		IsPrivate:            ptr.To(isPrivate),
		IsArchived:           ptr.To(isArchived),
		Sort:                 sort,
		Order:                order,
		ListOptions: gitea_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(pageSize),
		},
	}
	repos, _, err := gitea.Client().SearchRepos(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search repos error: %v", err))
	}
	return to.TextResult(repos)
}
