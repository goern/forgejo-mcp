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
		mcp.WithString("keyword", mcp.Description("Keyword"), mcp.DefaultString("")),
		mcp.WithNumber("page", mcp.Description("Page"), mcp.DefaultNumber(1)),
		mcp.WithNumber("pageSize", mcp.Description("PageSize"), mcp.DefaultNumber(100)),
	)

	SearOrgTeamsTool = mcp.NewTool(
		SearchOrgTeamsToolName,
		mcp.WithDescription("search organization teams"),
		mcp.WithString("org", mcp.Description("organization name"), mcp.DefaultString("")),
		mcp.WithString("query", mcp.Description("search organization teams"), mcp.DefaultString("")),
		mcp.WithBoolean("includeDescription", mcp.Description("include description?"), mcp.DefaultBool(true)),
		mcp.WithNumber("page", mcp.Description("Page"), mcp.DefaultNumber(1)),
		mcp.WithNumber("pageSize", mcp.Description("PageSize"), mcp.DefaultNumber(100)),
	)

	SearchReposTool = mcp.NewTool(
		SearchReposToolName,
		mcp.WithDescription("search repos"),
		mcp.WithString("keyword", mcp.Description("Keyword"), mcp.DefaultString("")),
		mcp.WithBoolean("keywordIsTopic", mcp.Description("KeywordIsTopic"), mcp.DefaultBool(false)),
		mcp.WithBoolean("keywordInDescription", mcp.Description("KeywordInDescription"), mcp.DefaultBool(false)),
		mcp.WithNumber("ownerID", mcp.Description("OwnerID"), mcp.DefaultNumber(0)),
		mcp.WithBoolean("isPrivate", mcp.Description("IsPrivate"), mcp.DefaultBool(false)),
		mcp.WithBoolean("isArchived", mcp.Description("IsArchived"), mcp.DefaultBool(false)),
		mcp.WithString("sort", mcp.Description("Sort"), mcp.DefaultString(""), mcp.Enum("")),
		mcp.WithString("order", mcp.Description("Order"), mcp.DefaultString(""), mcp.Enum("")),
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
	opt := gitea_sdk.SearchUsersOption{
		KeyWord: req.Params.Arguments["keyword"].(string),
		ListOptions: gitea_sdk.ListOptions{
			Page:     req.Params.Arguments["page"].(int),
			PageSize: req.Params.Arguments["pageSize"].(int),
		},
	}
	users, _, err := gitea.Client().SearchUsers(opt)
	if err != nil {
		return nil, err
	}
	return to.TextResult(users)
}

func SearchOrgTeamsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchOrgTeamsFn")
	org := req.Params.Arguments["org"].(string)
	opt := gitea_sdk.SearchTeamsOptions{
		Query:              req.Params.Arguments["query"].(string),
		IncludeDescription: req.Params.Arguments["includeDescription"].(bool),
		ListOptions: gitea_sdk.ListOptions{
			Page:     req.Params.Arguments["page"].(int),
			PageSize: req.Params.Arguments["pageSize"].(int),
		},
	}
	teams, _, err := gitea.Client().SearchOrgTeams(org, &opt)
	if err != nil {
		return nil, fmt.Errorf("search organization teams error: %v", err)
	}
	return to.TextResult(teams)
}

func SearchReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchReposFn")
	opt := gitea_sdk.SearchRepoOptions{
		Keyword:              req.Params.Arguments["keyword"].(string),
		KeywordIsTopic:       req.Params.Arguments["keywordIsTopic"].(bool),
		KeywordInDescription: req.Params.Arguments["keywordInDescription"].(bool),
		OwnerID:              req.Params.Arguments["ownerID"].(int64),
		IsPrivate:            ptr.To(req.Params.Arguments["isPrivate"].(bool)),
		IsArchived:           ptr.To(req.Params.Arguments["isArchived"].(bool)),
		Sort:                 req.Params.Arguments["sort"].(string),
		Order:                req.Params.Arguments["order"].(string),
		ListOptions: gitea_sdk.ListOptions{
			Page:     req.Params.Arguments["page"].(int),
			PageSize: req.Params.Arguments["pageSize"].(int),
		},
	}
	repos, _, err := gitea.Client().SearchRepos(opt)
	if err != nil {
		return nil, fmt.Errorf("search repos error: %v", err)
	}
	return to.TextResult(repos)
}
