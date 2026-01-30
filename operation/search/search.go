package search

import (
	"context"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
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
		mcp.WithDescription("Search users"),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchOrgTeamsTool = mcp.NewTool(
		SearchOrgTeamsToolName,
		mcp.WithDescription("Search org teams"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchReposTool = mcp.NewTool(
		SearchReposToolName,
		mcp.WithDescription("Search repos"),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithString("sort", mcp.Description(params.Sort), mcp.DefaultString("updated")),
		mcp.WithString("order", mcp.Description(params.Order), mcp.DefaultString("desc")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(SearchUsersTool, SearchUserFn)
	s.AddTool(SearchOrgTeamsTool, SearchOrgTeamsFn)
	s.AddTool(SearchReposTool, SearchReposFn)
}

func SearchUserFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Create a search query for dummy implementation
	keyword, _ := req.GetArguments()["keyword"].(string)
	
	// Create a basic search option with just a keyword
	opt := forgejo_sdk.SearchUsersOption{
		KeyWord: keyword,
	}

	// Use the correct options type for searching
	result, _, err := forgejo.Client().SearchUsers(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search user err: %v", err))
	}
	return to.TextResult(result)
}

func SearchOrgTeamsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Create basic search teams options
	log.Debugf("Called SearchOrgTeamsFn")
	org, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("org name is required"))
	}

	keyword, _ := req.GetArguments()["keyword"].(string)
	
	// Create proper search team options
	opt := &forgejo_sdk.SearchTeamsOptions{
		Query: keyword,
	}

	// Use the proper options type for search
	result, _, err := forgejo.Client().SearchOrgTeams(org, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search org teams err: %v", err))
	}
	return to.TextResult(result)
}

func SearchReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchReposFn")
	keyword, _ := req.GetArguments()["keyword"].(string)
	sort, _ := req.GetArguments()["sort"].(string)
	order, _ := req.GetArguments()["order"].(string)
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 100
	}

	// Create a proper search options structure
	opt := forgejo_sdk.SearchRepoOptions{
		Keyword: keyword,
		Sort:    sort,
		Order:   order,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	
	// Call search repos with proper options
	result, _, err := forgejo.Client().SearchRepos(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search repos err: %v", err))
	}
	return to.TextResult(result)
}