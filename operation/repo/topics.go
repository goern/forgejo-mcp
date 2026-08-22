// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoTopicsToolName  = "list_repo_topics"
	SetRepoTopicsToolName   = "set_repo_topics"
	AddRepoTopicToolName    = "add_repo_topic"
	DeleteRepoTopicToolName = "delete_repo_topic"

	maxRepoTopics = 25
	maxTopicLen   = 35
)

var topicNameRe = regexp.MustCompile(`^[a-z0-9][-.a-z0-9]*$`)

var (
	ListRepoTopicsTool = mcp.NewTool(
		ListRepoTopicsToolName,
		mcp.WithDescription("List a repository's topics. Bounded by page (default 1) and limit (default 100); returns {topics, page, limit, count}."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	SetRepoTopicsTool = mcp.NewTool(
		SetRepoTopicsToolName,
		mcp.WithDescription("Replace all repository topics (comma-separated). The topics key is required; an empty string clears every topic. Names are lowercased. At most 25 topics, each matching ^[a-z0-9][-.a-z0-9]*$ and at most 35 characters."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("topics", mcp.Required(), mcp.Description("Repository topics (comma-separated). Empty string clears all topics.")),
	)

	AddRepoTopicTool = mcp.NewTool(
		AddRepoTopicToolName,
		mcp.WithDescription("Add one topic to a repository. The name is lowercased and must match Forgejo's topic rules."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Topic name")),
	)

	DeleteRepoTopicTool = mcp.NewTool(
		DeleteRepoTopicToolName,
		mcp.WithDescription("Delete one topic from a repository."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Topic name")),
	)
)

type listRepoTopicsResult struct {
	Topics []string `json:"topics"`
	Page   int      `json:"page"`
	Limit  int      `json:"limit"`
	Count  int      `json:"count"`
}

func normalizeTopic(s string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	if name == "" {
		return "", fmt.Errorf("topic name is empty")
	}
	if len(name) > maxTopicLen {
		return "", fmt.Errorf("topic %q exceeds %d characters", name, maxTopicLen)
	}
	if !topicNameRe.MatchString(name) {
		return "", fmt.Errorf("topic %q is invalid: must match ^[a-z0-9][-.a-z0-9]*$", name)
	}
	return name, nil
}

func splitTopics(s string) ([]string, error) {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		name, err := normalizeTopic(part)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) > maxRepoTopics {
		return nil, fmt.Errorf("at most %d topics per repository, got %d", maxRepoTopics, len(out))
	}
	return out, nil
}

func ListRepoTopicsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoTopicsFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 100
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	topics, _, err := client.ListRepoTopics(owner, repo, forgejo_sdk.ListRepoTopicsOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: int(page), PageSize: int(limit)},
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo topics: %w", err))
	}
	if topics == nil {
		topics = []string{}
	}
	return to.TextResult(listRepoTopicsResult{
		Topics: topics,
		Page:   int(page),
		Limit:  int(limit),
		Count:  len(topics),
	})
}

func SetRepoTopicsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SetRepoTopicsFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}
	raw, ok := args["topics"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("topics is required"))
	}
	topics, err := splitTopics(raw)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.SetRepoTopics(owner, repo, topics); err != nil {
		return to.ErrorResult(fmt.Errorf("set repo topics: %w", err))
	}
	return to.TextResult(map[string]any{"topics": topics})
}

func AddRepoTopicFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddRepoTopicFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}
	raw, _ := args["topic"].(string)
	name, err := normalizeTopic(raw)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.AddRepoTopic(owner, repo, name); err != nil {
		return to.ErrorResult(fmt.Errorf("add repo topic: %w", err))
	}
	return to.TextResult(map[string]string{"topic": name, "status": "added"})
}

func DeleteRepoTopicFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteRepoTopicFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	if owner == "" || repo == "" {
		return to.ErrorResult(fmt.Errorf("owner and repo are required"))
	}
	raw, _ := args["topic"].(string)
	name, err := normalizeTopic(raw)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.DeleteRepoTopic(owner, repo, name); err != nil {
		return to.ErrorResult(fmt.Errorf("delete repo topic: %w", err))
	}
	return to.TextResult(map[string]string{"topic": name, "status": "deleted"})
}
