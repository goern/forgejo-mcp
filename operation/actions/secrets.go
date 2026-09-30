// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"errors"
	"fmt"
	"time"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoActionSecretsToolName          = "list_repo_action_secrets"
	CreateOrUpdateRepoActionSecretToolName = "create_or_update_repo_action_secret"
	DeleteRepoActionSecretToolName         = "delete_repo_action_secret"
	ListOrgActionSecretsToolName           = "list_org_action_secrets"
	CreateOrUpdateOrgActionSecretToolName  = "create_or_update_org_action_secret"
	DeleteOrgActionSecretToolName          = "delete_org_action_secret"

	defaultActionSecretsLimit = 30
	maxActionSecretsLimit     = 50
)

// actionSecretSummary intentionally has no Data field. Forgejo never returns
// a secret's value over the API, but omitting the field here too means a
// future upstream change (or a bug in the SDK) can't leak one through this
// tool by accident.
type actionSecretSummary struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
}

type listRepoActionSecretsResult struct {
	Owner   string                `json:"owner"`
	Repo    string                `json:"repo"`
	Secrets []actionSecretSummary `json:"secrets"`
	Page    int                   `json:"page"`
	Limit   int                   `json:"limit"`
	Count   int                   `json:"count"`
}

type listOrgActionSecretsResult struct {
	Org     string                `json:"org"`
	Secrets []actionSecretSummary `json:"secrets"`
	Page    int                   `json:"page"`
	Limit   int                   `json:"limit"`
	Count   int                   `json:"count"`
}

type actionSecretMutationResult struct {
	Status     string `json:"status"`
	Owner      string `json:"owner,omitempty"`
	Repo       string `json:"repo,omitempty"`
	Org        string `json:"org,omitempty"`
	SecretName string `json:"secret_name"`
}

var (
	ListRepoActionSecretsTool = mcp.NewTool(
		ListRepoActionSecretsToolName,
		mcp.WithDescription("List action secrets for a repository (names only, values are never exposed by the Forgejo API)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionSecretsLimit), mcp.Min(1), mcp.Max(maxActionSecretsLimit)),
	)

	CreateOrUpdateRepoActionSecretTool = mcp.NewTool(
		CreateOrUpdateRepoActionSecretToolName,
		mcp.WithDescription("Create or update an action secret for a repository. Overwrites any existing secret with the same name; the value is never echoed back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.SecretData)),
	)

	DeleteRepoActionSecretTool = mcp.NewTool(
		DeleteRepoActionSecretToolName,
		mcp.WithDescription("Delete an action secret from a repository."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
	)

	ListOrgActionSecretsTool = mcp.NewTool(
		ListOrgActionSecretsToolName,
		mcp.WithDescription("List action secrets for an organization (names only, values are never exposed by the Forgejo API)."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionSecretsLimit), mcp.Min(1), mcp.Max(maxActionSecretsLimit)),
	)

	CreateOrUpdateOrgActionSecretTool = mcp.NewTool(
		CreateOrUpdateOrgActionSecretToolName,
		mcp.WithDescription("Create or update an action secret for an organization. Overwrites any existing secret with the same name; the value is never echoed back."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.SecretData)),
	)

	DeleteOrgActionSecretTool = mcp.NewTool(
		DeleteOrgActionSecretToolName,
		mcp.WithDescription("Delete an action secret from an organization."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
	)
)

func requiredOrg(args map[string]any) (string, error) {
	org, ok := args["org"].(string)
	if !ok || org == "" {
		return "", errors.New("org is required")
	}
	return org, nil
}

func requiredSecretName(args map[string]any) (string, error) {
	name, ok := args["secret_name"].(string)
	if !ok || name == "" {
		return "", errors.New("secret_name is required")
	}
	return name, nil
}

func requiredSecretData(args map[string]any) (string, error) {
	data, ok := args["data"].(string)
	if !ok || data == "" {
		return "", errors.New("data is required")
	}
	return data, nil
}

func toSecretSummaries(secrets []*forgejo_sdk.Secret) []actionSecretSummary {
	summaries := make([]actionSecretSummary, 0, len(secrets))
	for _, s := range secrets {
		summary := actionSecretSummary{Name: s.Name}
		if !s.Created.IsZero() {
			summary.CreatedAt = s.Created.Format(time.RFC3339)
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func ListRepoActionSecretsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoActionSecretsFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntegerArg(args, "page", 1, 1, 1<<31-1)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntegerArg(args, "limit", defaultActionSecretsLimit, 1, maxActionSecretsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	secrets, _, err := client.ListRepoActionSecret(owner, repo, forgejo_sdk.ListRepoActionSecretOption{
		ListOptions: forgejo_sdk.ListOptions{Page: page, PageSize: limit},
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo action secrets: %w", err))
	}

	summaries := toSecretSummaries(secrets)
	return to.SafeTextResult(listRepoActionSecretsResult{
		Owner:   owner,
		Repo:    repo,
		Secrets: summaries,
		Page:    page,
		Limit:   limit,
		Count:   len(summaries),
	})
}

func CreateOrUpdateRepoActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrUpdateRepoActionSecretFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	secretName, err := requiredSecretName(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	data, err := requiredSecretData(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.CreateRepoActionSecret(owner, repo, forgejo_sdk.CreateSecretOption{Name: secretName, Data: data}); err != nil {
		return to.ErrorResult(fmt.Errorf("create or update repo action secret: %w", err))
	}

	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "created_or_updated",
		Owner:      owner,
		Repo:       repo,
		SecretName: secretName,
	})
}

func DeleteRepoActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteRepoActionSecretFn")
	args := req.GetArguments()

	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	secretName, err := requiredSecretName(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.DeleteRepoActionSecret(owner, repo, secretName); err != nil {
		return to.ErrorResult(fmt.Errorf("delete repo action secret: %w", err))
	}

	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "deleted",
		Owner:      owner,
		Repo:       repo,
		SecretName: secretName,
	})
}

func ListOrgActionSecretsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListOrgActionSecretsFn")
	args := req.GetArguments()

	org, err := requiredOrg(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntegerArg(args, "page", 1, 1, 1<<31-1)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntegerArg(args, "limit", defaultActionSecretsLimit, 1, maxActionSecretsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	secrets, _, err := client.ListOrgActionSecret(org, forgejo_sdk.ListOrgActionSecretOption{
		ListOptions: forgejo_sdk.ListOptions{Page: page, PageSize: limit},
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list org action secrets: %w", err))
	}

	summaries := toSecretSummaries(secrets)
	return to.SafeTextResult(listOrgActionSecretsResult{
		Org:     org,
		Secrets: summaries,
		Page:    page,
		Limit:   limit,
		Count:   len(summaries),
	})
}

func CreateOrUpdateOrgActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrUpdateOrgActionSecretFn")
	args := req.GetArguments()

	org, err := requiredOrg(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	secretName, err := requiredSecretName(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	data, err := requiredSecretData(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.CreateOrgActionSecret(org, forgejo_sdk.CreateSecretOption{Name: secretName, Data: data}); err != nil {
		return to.ErrorResult(fmt.Errorf("create or update org action secret: %w", err))
	}

	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "created_or_updated",
		Org:        org,
		SecretName: secretName,
	})
}

func DeleteOrgActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteOrgActionSecretFn")
	args := req.GetArguments()

	org, err := requiredOrg(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	secretName, err := requiredSecretName(args)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if _, err := client.DeleteOrgActionSecret(org, secretName); err != nil {
		return to.ErrorResult(fmt.Errorf("delete org action secret: %w", err))
	}

	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "deleted",
		Org:        org,
		SecretName: secretName,
	})
}
